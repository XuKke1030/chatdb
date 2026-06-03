package admin

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/logic/aidgp"
	"ai-chat-sql/internal/model"
	"ai-chat-sql/internal/service"
	"ai-chat-sql/utility"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	v1 "ai-chat-sql/api/admin/v1"

	"github.com/extrame/xls"
	"github.com/gogf/gf/v2/crypto/gmd5"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/xuri/excelize/v2"
)

func (c *ControllerV1) AdminLogin(ctx context.Context, req *v1.AdminLoginReq) (res *v1.AdminLoginRes, err error) {
	if req.Username == "" || req.Password == "" {
		return nil, gerror.New("管理员账号和密码不能为空")
	}
	record, err := g.DB("master").Model("admin_account").Ctx(ctx).Where("username = ? AND enabled = ?", req.Username, 1).One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, gerror.New("管理员账号或密码错误")
	}
	verify := record["verify"].Int()
	password, err := genAdminPassword(req.Password, verify)
	if err != nil {
		return nil, err
	}
	if record["password"].String() != password {
		return nil, gerror.New("管理员账号或密码错误")
	}
	_ = insertAdminLog(ctx, "admin", req.Username, "管理员登录", "管理员登录后台管理平台", "success")
	jwtOut, jwtErr := service.Jwt().GenToken(ctx, &model.JWTGenTokenInput{
		Subject: consts.JwtSubjectAdmin,
		Id:      int64(record["id"].Int()),
	})
	if jwtErr != nil {
		return nil, jwtErr
	}
	// Also generate a user JWT so the admin can access user-scoped endpoints (e.g. QA sync)
	var userToken string
	userRecord, _ := g.DB("master").Model("user").Ctx(ctx).Fields("user_id").Where("username = ?", req.Username).One()
	if userRecord != nil && userRecord["user_id"].Int() > 0 {
		if userOut, userErr := service.Jwt().GenToken(ctx, &model.JWTGenTokenInput{
			Subject: consts.JwtSubjectUser,
			Id:      int64(userRecord["user_id"].Int()),
		}); userErr == nil && userOut != nil {
			userToken = userOut.Token
		}
	}
	return &v1.AdminLoginRes{
		Token:     jwtOut.Token,
		Username:  req.Username,
		UserToken: userToken,
	}, nil
}

func (c *ControllerV1) AdminProfile(ctx context.Context, req *v1.AdminProfileReq) (res *v1.AdminProfileRes, err error) {
	adminId := model.UserIdFromContext(ctx)
	if adminId > 0 {
		record, dbErr := g.DB("master").Model("admin_account").Ctx(ctx).Where("id = ?", adminId).One()
		if dbErr == nil && record != nil {
			return &v1.AdminProfileRes{
				Username: record["username"].String(),
				Role:     record["role"].String(),
			}, nil
		}
	}
	return &v1.AdminProfileRes{Username: "admin", Role: "administrator"}, nil
}

func (c *ControllerV1) AdminUsers(ctx context.Context, req *v1.AdminUsersReq) (res *v1.AdminUsersRes, err error) {
	users, total, err := listAdminUsers(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUsersRes{List: users, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (c *ControllerV1) AdminKnowledgeBases(ctx context.Context, req *v1.AdminKnowledgeBasesReq) (res *v1.AdminKnowledgeBasesRes, err error) {
	all := knowledgeBaseOptions(ctx)
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return &v1.AdminKnowledgeBasesRes{List: all[start:end], Total: total, Page: page, PageSize: pageSize}, nil
}

func (c *ControllerV1) AdminUpdateUserPermissions(ctx context.Context, req *v1.AdminUpdateUserPermissionsReq) (res *v1.AdminUpdateUserPermissionsRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("用户ID不能为空")
	}
	ruleLevel := req.RuleLevel
	if ruleLevel == 0 && len(req.Permissions) > 0 {
		ruleLevel = ruleLevelFromPermissions(req.Permissions)
	}
	now := int(gtime.Timestamp())
	if _, err := g.DB("master").Model("user").Ctx(ctx).Where("user_id = ?", req.Id).Data(g.Map{
		"rule_level":  ruleLevel,
		"update_time": now,
	}).Update(); err != nil {
		return nil, gerror.Wrap(err, "更新用户权限等级失败")
	}
	if err = upsertUserProfile(ctx, req.Id, g.Map{"rule_level": ruleLevel, "update_time": now}); err != nil {
		return nil, err
	}
	if err = replaceUserKnowledgePermissions(ctx, req.Id, req.QaPermissions); err != nil {
		return nil, err
	}
	if err = bumpUserPermissionVersion(ctx, req.Id); err != nil {
		return nil, err
	}
	clearUserPermissionCache(ctx, req.Id)
	_ = insertAdminLog(ctx, "admin", "admin", "权限更新", fmt.Sprintf("更新用户ID %d 的问数/问答权限", req.Id), "success")
	user, err := getAdminUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUpdateUserPermissionsRes{User: user}, nil
}

func (c *ControllerV1) AdminUpdateUserStatus(ctx context.Context, req *v1.AdminUpdateUserStatusReq) (res *v1.AdminUpdateUserStatusRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("用户ID不能为空")
	}
	if err = upsertUserProfile(ctx, req.Id, g.Map{"enabled": boolToInt(req.Enabled), "update_time": int(gtime.Timestamp())}); err != nil {
		return nil, err
	}
	if err = bumpUserPermissionVersion(ctx, req.Id); err != nil {
		return nil, err
	}
	clearUserPermissionCache(ctx, req.Id)
	_ = insertAdminLog(ctx, "admin", "admin", "账号状态", fmt.Sprintf("更新用户ID %d 状态为 %v", req.Id, req.Enabled), "success")
	user, err := getAdminUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUpdateUserStatusRes{User: user}, nil
}

func (c *ControllerV1) AdminExampleQuestions(ctx context.Context, req *v1.AdminExampleQuestionsReq) (res *v1.AdminExampleQuestionsRes, err error) {
	query := g.DB("master").Model("admin_example_question").Ctx(ctx)
	if req.Topic != "" {
		query = query.Where("topic", req.Topic)
	}
	total, err := query.Count()
	if err != nil {
		return nil, err
	}
	records, err := query.OrderAsc("sort").OrderDesc("update_time").Limit(req.PageSize).Offset((req.Page-1)*req.PageSize).All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminExampleQuestionsRes{List: scanExampleQuestions(records), Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (c *ControllerV1) AdminCreateExampleQuestion(ctx context.Context, req *v1.AdminCreateExampleQuestionReq) (res *v1.AdminCreateExampleQuestionRes, err error) {
	if req.Topic == "" || req.Question == "" {
		return nil, gerror.New("topic and question are required")
	}
	now := int(gtime.Timestamp())
	id, err := g.DB("master").Model("admin_example_question").Ctx(ctx).Data(g.Map{
		"topic":       req.Topic,
		"question":    req.Question,
		"description": req.Description,
		"enabled":     boolToInt(req.Enabled),
		"sort":        req.Sort,
		"create_time": now,
		"update_time": now,
	}).InsertAndGetId()
	if err != nil {
		return nil, err
	}
	item, err := getExampleQuestion(ctx, int(id))
	if err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", "admin", "示例问题", fmt.Sprintf("新增示例问题：%s", req.Question), "success")
	return &v1.AdminCreateExampleQuestionRes{Item: item}, nil
}

func (c *ControllerV1) AdminUpdateExampleQuestion(ctx context.Context, req *v1.AdminUpdateExampleQuestionReq) (res *v1.AdminUpdateExampleQuestionRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("问题ID不能为空")
	}
	_, err = g.DB("master").Model("admin_example_question").Ctx(ctx).Where("id = ?", req.Id).Data(g.Map{
		"topic":       req.Topic,
		"question":    req.Question,
		"description": req.Description,
		"enabled":     boolToInt(req.Enabled),
		"sort":        req.Sort,
		"update_time": int(gtime.Timestamp()),
	}).Update()
	if err != nil {
		return nil, err
	}
	item, err := getExampleQuestion(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", "admin", "示例问题", fmt.Sprintf("更新示例问题ID %d", req.Id), "success")
	return &v1.AdminUpdateExampleQuestionRes{Item: item}, nil
}

func (c *ControllerV1) AdminDeleteExampleQuestion(ctx context.Context, req *v1.AdminDeleteExampleQuestionReq) (res *v1.AdminDeleteExampleQuestionRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("问题ID不能为空")
	}
	_, err = g.DB("master").Model("admin_example_question").Ctx(ctx).Where("id = ?", req.Id).Delete()
	_ = insertAdminLog(ctx, "admin", "admin", "示例问题", fmt.Sprintf("删除示例问题ID %d", req.Id), "success")
	return &v1.AdminDeleteExampleQuestionRes{}, err
}

func (c *ControllerV1) AdminQuestionCandidates(ctx context.Context, req *v1.AdminQuestionCandidatesReq) (res *v1.AdminQuestionCandidatesRes, err error) {
	query := g.DB("master").Model("admin_question_candidate").Ctx(ctx)
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}
	total, err := query.Count()
	if err != nil {
		return nil, err
	}
	records, err := query.OrderDesc("count").OrderDesc("last_seen_at").Limit(req.PageSize).Offset((req.Page-1)*req.PageSize).All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminQuestionCandidatesRes{List: scanQuestionCandidates(records), Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (c *ControllerV1) AdminApproveQuestionCandidate(ctx context.Context, req *v1.AdminApproveQuestionCandidateReq) (res *v1.AdminApproveQuestionCandidateRes, err error) {
	item, err := updateQuestionCandidateStatus(ctx, req.Id, "approved")
	if err != nil {
		return nil, err
	}
	// Auto-create example question from approved candidate
	if item.Question != "" {
		now := int(gtime.Timestamp())
		existing, countErr := g.DB("master").Model("admin_example_question").Ctx(ctx).
			Where("topic = ? AND question = ?", item.Topic, item.Question).Count()
		if countErr != nil {
			consts.Logger.Warningf(ctx, "查询示例问题失败: %s", countErr.Error())
		}
		if existing == 0 {
			if _, insertErr := g.DB("master").Model("admin_example_question").Ctx(ctx).Data(g.Map{
				"topic":       item.Topic,
				"question":    item.Question,
				"description": fmt.Sprintf("从用户提问自动沉淀（累计%d次）", item.Count),
				"enabled":     1,
				"sort":        0,
				"create_time": now,
				"update_time": now,
			}).Insert(); insertErr != nil {
				consts.Logger.Warningf(ctx, "自动沉淀示例问题失败: %s", insertErr.Error())
			}
		}
	}
	return &v1.AdminApproveQuestionCandidateRes{Item: item}, nil
}

func (c *ControllerV1) AdminRejectQuestionCandidate(ctx context.Context, req *v1.AdminRejectQuestionCandidateReq) (res *v1.AdminRejectQuestionCandidateRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("候选问题ID不能为空")
	}
	item, err := updateQuestionCandidateStatus(ctx, req.Id, "rejected")
	if err != nil {
		return nil, err
	}
	return &v1.AdminRejectQuestionCandidateRes{Item: item}, nil
}

func (c *ControllerV1) AdminDataSources(ctx context.Context, req *v1.AdminDataSourcesReq) (res *v1.AdminDataSourcesRes, err error) {
	query := g.DB("master").Model("admin_data_source").Ctx(ctx)
	total, err := query.Count()
	if err != nil {
		return nil, err
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	records, err := query.OrderAsc("id").Limit(pageSize).Offset((page-1)*pageSize).All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminDataSourcesRes{List: scanDataSources(records), Total: total, Page: page, PageSize: pageSize}, nil
}

func (c *ControllerV1) AdminUpdateDataSource(ctx context.Context, req *v1.AdminUpdateDataSourceReq) (res *v1.AdminUpdateDataSourceRes, err error) {
	if req.Type == "" {
		return nil, gerror.New("data source type is required")
	}
	status := req.Status
	if status == "" {
		if req.Enabled {
			status = "running"
		} else {
			status = "closed"
		}
	}
	now := int(gtime.Timestamp())
	_, err = g.DB("master").Model("admin_data_source").Ctx(ctx).Where("source_type = ?", req.Type).Data(g.Map{
		"enabled":     boolToInt(req.Enabled),
		"status":      status,
		"update_time": now,
	}).Update()
	if err != nil {
		return nil, err
	}
	item, err := getDataSource(ctx, req.Type)
	if err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", "admin", "数据接入", fmt.Sprintf("更新%s数据源状态为%s", req.Type, status), "success")
	return &v1.AdminUpdateDataSourceRes{Item: item}, nil
}

func (c *ControllerV1) AdminGridImportUpload(ctx context.Context, req *v1.AdminGridImportUploadReq) (res *v1.AdminGridImportUploadRes, err error) {

	fileName := strings.TrimSpace(req.FileName)
	month := strings.TrimSpace(req.Month)
	operator := defaultString(req.Operator, "admin")
	cases := []gridCaseRecord{}
	parseErrors := []gridParseError{}
	if request := ghttp.RequestFromCtx(ctx); request != nil {
		if month == "" {
			month = request.Get("month").String()
		}
		if operator == "admin" && request.Get("operator").String() != "" {
			operator = request.Get("operator").String()
		}
		if uploadFile := request.GetUploadFile("file"); uploadFile != nil {
			fileName = uploadFile.Filename
			file, openErr := uploadFile.Open()
			if openErr != nil {
				return nil, openErr
			}
			defer file.Close()
			content, readErr := io.ReadAll(file)
			if readErr != nil {
				return nil, readErr
			}
			cases, parseErrors, err = parseGridImportFile(fileName, content)
			if err != nil {
				return nil, err
			}
		}
	}
	if fileName == "" {
		return nil, gerror.New("请选择要上传的 .xlsx / .xls / .csv 文件")
	}

	totalRows := len(cases) + len(parseErrors)
	if totalRows == 0 && req.TotalRows > 0 {
		totalRows = req.TotalRows
	}
	failedRows := len(parseErrors)
	successRows := totalRows - failedRows
	if successRows < 0 {
		successRows = 0
	}
	status := "completed"
	if failedRows > 0 && successRows > 0 {
		status = "partial_success"
	} else if failedRows > 0 && successRows == 0 {
		status = "failed"
	}
	now := int(gtime.Timestamp())
	db := g.DB("master")
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	id, err := tx.Model("admin_grid_import").Ctx(ctx).Data(g.Map{
		"month":         month,
		"file_name":     fileName,
		"status":        status,
		"total_rows":    totalRows,
		"success_rows":  successRows,
		"failed_rows":   failedRows,
		"operator":      operator,
		"create_time":   now,
		"complete_time": now,
	}).InsertAndGetId()
	if err != nil {
		return nil, err
	}
	for _, item := range cases {
		if item.CaseNumber != "" {
			if _, err = tx.Model("case_list").Ctx(ctx).Where("case_number = ?", item.CaseNumber).Delete(); err != nil {
				return nil, err
			}
		}
		if _, err = tx.Model("case_list").Ctx(ctx).Data(item.toMap()).Insert(); err != nil {
			return nil, err
		}
	}
	for _, parseErr := range parseErrors {
		if _, err = tx.Model("admin_grid_import_error").Ctx(ctx).Data(g.Map{
			"import_id": id,
			"row_index": parseErr.RowIndex,
			"reason":    parseErr.Reason,
			"raw_data":  parseErr.RawData,
		}).Insert(); err != nil {
			return nil, err
		}
	}
	// Record imported case numbers for rollback lookup
	caseNums := make([]string, 0, len(cases))
	for _, item := range cases {
		if item.CaseNumber != "" {
			caseNums = append(caseNums, item.CaseNumber)
		}
	}
	if len(caseNums) > 0 {
		if _, err = tx.Model("admin_grid_import_error").Ctx(ctx).Data(g.Map{
			"import_id": id,
			"row_index": 0,
			"reason":    "rollback_case_numbers",
			"raw_data":  strings.Join(caseNums, ","),
		}).Insert(); err != nil {
			return nil, err
		}
	}
	// Audit: upload action
	if _, err = tx.Model("admin_grid_import_audit").Ctx(ctx).Data(g.Map{
		"import_id":   id,
		"action":      "upload",
		"operator":    defaultString(req.Operator, "admin"),
		"detail":      fmt.Sprintf("上传文件 %s，成功%d条，失败%d条", fileName, successRows, failedRows),
		"create_time": now,
	}).Insert(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	item, err := getGridImport(ctx, int(id))
	if err != nil {
		return nil, err
	}
	_ = insertAdminLog(ctx, "admin", req.Operator, "数据导入", fmt.Sprintf("上传网格数据：%s，成功%d条，失败%d条", fileName, successRows, failedRows), item.Status)
	return &v1.AdminGridImportUploadRes{Item: item}, nil
}

func (c *ControllerV1) AdminGridImports(ctx context.Context, req *v1.AdminGridImportsReq) (res *v1.AdminGridImportsRes, err error) {
	query := g.DB("master").Model("admin_grid_import").Ctx(ctx)
	total, err := query.Count()
	if err != nil {
		return nil, err
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	records, err := query.OrderDesc("create_time").Limit(pageSize).Offset((page-1)*pageSize).All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportsRes{List: scanGridImports(records), Total: total, Page: page, PageSize: pageSize}, nil
}

func (c *ControllerV1) AdminGridImportDetail(ctx context.Context, req *v1.AdminGridImportDetailReq) (res *v1.AdminGridImportDetailRes, err error) {
	item, err := getGridImport(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportDetailRes{Item: item}, nil
}

func (c *ControllerV1) AdminGridImportErrors(ctx context.Context, req *v1.AdminGridImportErrorsReq) (res *v1.AdminGridImportErrorsRes, err error) {
	records, err := g.DB("master").Model("admin_grid_import_error").Ctx(ctx).Where("import_id = ?", req.Id).OrderAsc("row_index").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportErrorsRes{List: scanGridImportErrors(records)}, nil
}

func (c *ControllerV1) AdminSyncKnowledgeBases(ctx context.Context, req *v1.AdminSyncKnowledgeBasesReq) (res *v1.AdminSyncRes, err error) {
	return executeAdminAidgpSync(ctx, aidgp.SyncKnowledgeBases, "")
}

func (c *ControllerV1) AdminSyncDocuments(ctx context.Context, req *v1.AdminSyncDocumentsReq) (res *v1.AdminSyncRes, err error) {
	return executeAdminAidgpSync(ctx, aidgp.SyncDocuments, strings.TrimSpace(req.KnowledgeCode))
}

func (c *ControllerV1) AdminSyncGridData(ctx context.Context, req *v1.AdminSyncGridDataReq) (res *v1.AdminSyncRes, err error) {
	return executeAdminAidgpSync(ctx, aidgp.SyncGridData, "")
}

func (c *ControllerV1) AdminSyncTrafficData(ctx context.Context, req *v1.AdminSyncTrafficDataReq) (res *v1.AdminSyncRes, err error) {
	return executeAdminAidgpSync(ctx, aidgp.SyncTrafficData, "")
}

func (c *ControllerV1) AdminRefreshTrafficAggregates(ctx context.Context, req *v1.AdminRefreshTrafficAggregatesReq) (res *v1.AdminRefreshTrafficAggregatesRes, err error) {
	if err := service.Traffic().RefreshAggregates(ctx, req.DateFrom, req.DateTo); err != nil {
		return nil, err
	}
	return &v1.AdminRefreshTrafficAggregatesRes{Ok: true}, nil
}

func (c *ControllerV1) AdminRefreshPopulationAggregates(ctx context.Context, req *v1.AdminRefreshPopulationAggregatesReq) (res *v1.AdminRefreshPopulationAggregatesRes, err error) {
	if err := service.Population().RefreshAggregates(ctx, req.DateFrom, req.DateTo); err != nil {
		return nil, err
	}
	return &v1.AdminRefreshPopulationAggregatesRes{Ok: true}, nil
}

func (c *ControllerV1) AdminSyncHoliday(ctx context.Context, req *v1.AdminSyncHolidayReq) (res *v1.AdminSyncHolidayRes, err error) {
	year := gtime.Now().Year()
	_ = service.Traffic().FetchHolidaysFromAPI(ctx, year)
	_ = service.Traffic().FetchHolidaysFromAPI(ctx, year+1)
	if err := service.Traffic().SyncHolidaysFromCode(ctx); err != nil {
		return nil, err
	}
	return &v1.AdminSyncHolidayRes{Ok: true}, nil
}

func (c *ControllerV1) AdminPlateVerify(ctx context.Context, req *v1.AdminPlateVerifyReq) (res *v1.AdminPlateVerifyRes, err error) {
	report, err := service.Traffic().BatchVerify(ctx, req.SampleSize)
	if err != nil {
		return nil, err
	}
	return &v1.AdminPlateVerifyRes{VerifyReport: *report}, nil
}

func (c *ControllerV1) AdminSyncPopulationData(ctx context.Context, req *v1.AdminSyncPopulationDataReq) (res *v1.AdminSyncRes, err error) {
	return executeAdminAidgpSync(ctx, aidgp.SyncPopulationData, "")
}

func (c *ControllerV1) AdminSyncStatus(ctx context.Context, req *v1.AdminSyncStatusReq) (res *v1.AdminSyncStatusRes, err error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = adminSyncProvider()
	}
	query := g.DB("master").Model("qa_sync_task").Ctx(ctx).
		Fields("id, provider, sync_type, status, message, success_count, failure_count, skipped_count, started_at, finished_at, create_time, update_time")
	if provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if syncType := strings.TrimSpace(req.SyncType); syncType != "" {
		query = query.Where("sync_type = ?", syncType)
	}
	records, err := query.OrderDesc("id").Limit(limit).All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminSyncStatusRes{
		Provider: provider,
		Enabled:  provider == aidgp.ProviderAidgp || provider == aidgp.ProviderMock,
		List:     scanAdminSyncTasks(records),
	}, nil
}

func (c *ControllerV1) AdminSyncLogs(ctx context.Context, req *v1.AdminSyncLogsReq) (res *v1.AdminSyncLogsRes, err error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := g.DB("master").Model("qa_sync_log").Ctx(ctx).
		Fields("id, task_id, provider, sync_type, external_id, local_id, action, status, message, create_time").
		Where("task_id = ?", req.TaskId)
	if status := strings.TrimSpace(req.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	records, err := query.OrderAsc("id").Limit(limit).All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminSyncLogsRes{TaskId: req.TaskId, List: scanAdminSyncLogs(records)}, nil
}

func (c *ControllerV1) AdminSyncRetry(ctx context.Context, req *v1.AdminSyncRetryReq) (res *v1.AdminSyncRetryRes, err error) {
	record, err := g.DB("master").Model("qa_sync_task").Ctx(ctx).
		Fields("sync_type").
		Where("id = ?", req.TaskId).
		One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("sync task not found: %d", req.TaskId)
	}
	syncType := record["sync_type"].String()
	result, err := executeAdminAidgpSyncWithScope(ctx, syncType, "")
	if err != nil {
		return nil, err
	}
	out := v1.AdminSyncRetryRes(*result)
	return &out, nil
}

func (c *ControllerV1) AdminAidgpConnectionTest(ctx context.Context, req *v1.AdminAidgpConnectionTestReq) (res *v1.AdminAidgpConnectionTestRes, err error) {
	provider := adminSyncProvider()
	start := time.Now()
	client := adminAidgpClient(provider)
	testErr := client.TestConnection(ctx)
	res = &v1.AdminAidgpConnectionTestRes{
		Provider: provider,
		Success:  testErr == nil,
		CostMs:   time.Since(start).Milliseconds(),
	}
	if testErr != nil {
		res.Message = testErr.Error()
		return res, nil
	}
	res.Message = "AIDGP connection test passed"
	return res, nil
}

func (c *ControllerV1) AdminSyncFreshness(ctx context.Context, req *v1.AdminSyncFreshnessReq) (res *v1.AdminSyncFreshnessRes, err error) {
	types := []string{aidgp.SyncKnowledgeBases, aidgp.SyncDocuments, aidgp.SyncPermissions, aidgp.SyncGridData, aidgp.SyncTrafficData, aidgp.SyncPopulationData}
	list := make([]v1.AdminSyncFreshnessItem, 0, len(types))
	for _, syncType := range types {
		task, _ := g.DB("master").Model("qa_sync_task").Ctx(ctx).
			Fields("finished_at, status").
			Where("sync_type = ?", syncType).
			OrderDesc("id").
			One()
		raw, _ := g.DB("master").Model("aidgp_sync_record").Ctx(ctx).
			Fields("MAX(last_sync_time) AS latest_record_at, COUNT(*) AS record_count").
			Where("sync_type = ?", syncType).
			One()
		status := "none"
		latestTaskAt := 0
		if task != nil {
			status = task["status"].String()
			latestTaskAt = task["finished_at"].Int()
		}
		latestRecordAt := 0
		recordCount := 0
		if raw != nil {
			latestRecordAt = raw["latest_record_at"].Int()
			recordCount = raw["record_count"].Int()
		}
		list = append(list, v1.AdminSyncFreshnessItem{
			SyncType:       syncType,
			LatestTaskAt:   latestTaskAt,
			LatestRecordAt: latestRecordAt,
			RecordCount:    recordCount,
			Status:         status,
		})
	}
	return &v1.AdminSyncFreshnessRes{List: list}, nil
}

func (c *ControllerV1) AdminSyncReconcile(ctx context.Context, req *v1.AdminSyncReconcileReq) (res *v1.AdminSyncReconcileRes, err error) {
	items := []v1.AdminSyncReconcileItem{
		reconcileSyncType(ctx, aidgp.SyncKnowledgeBases, "qa_knowledge_base", "source_provider = 'aidgp'"),
		reconcileSyncType(ctx, aidgp.SyncDocuments, "qa_document", "source_provider = 'aidgp'"),
		reconcileSyncType(ctx, aidgp.SyncGridData, "grid_case_record", "source_provider = 'aidgp'"),
		reconcileSyncType(ctx, aidgp.SyncTrafficData, "traffic_gate_record", "1 = 1"),
		reconcileSyncType(ctx, aidgp.SyncPopulationData, "population_flow_record", "source_provider = 'aidgp'"),
	}
	return &v1.AdminSyncReconcileRes{List: items}, nil
}

func (c *ControllerV1) AdminLogs(ctx context.Context, req *v1.AdminLogsReq) (res *v1.AdminLogsRes, err error) {
	query := g.DB("master").Model("admin_operation_log").Ctx(ctx)
	if req.LogType == "system" || req.LogType == "admin" {
		query = query.Where("log_type = ?", req.LogType)
	}
	total, err := query.Count()
	if err != nil {
		return nil, err
	}
	records, err := query.OrderDesc("create_time").OrderDesc("id").Limit(req.PageSize).Offset((req.Page-1)*req.PageSize).All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminLogsRes{List: scanAdminLogs(records), Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (c *ControllerV1) updateCandidateStatus(ctx context.Context, id int, status string) (*v1.AdminApproveQuestionCandidateRes, error) {
	if id <= 0 {
		return nil, gerror.New("候选问题ID不能为空")
	}
	item, err := updateQuestionCandidateStatus(ctx, id, status)
	if err != nil {
		return nil, err
	}
	return &v1.AdminApproveQuestionCandidateRes{Item: item}, nil
}

func CreateAdminTables(ctx context.Context, db gdb.DB) error {
	for _, sql := range mysqlAdminTableSQL() {
		if _, err := db.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}

func MigrateAdminTables(ctx context.Context, db gdb.DB) {
	_, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN display_name VARCHAR(64)")
	_, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN last_login_at INT NOT NULL DEFAULT 0")
	_, _ = db.Exec(ctx, "ALTER TABLE admin_user_profile ADD COLUMN permission_version INT NOT NULL DEFAULT 1")
}

func mysqlAdminTableSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS admin_account (
id INT PRIMARY KEY AUTO_INCREMENT,
username VARCHAR(64) NOT NULL UNIQUE,
password VARCHAR(64) NOT NULL,
verify INT NOT NULL,
role VARCHAR(32) NOT NULL DEFAULT 'administrator',
enabled TINYINT NOT NULL DEFAULT 1,
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_user_profile (
user_id INT PRIMARY KEY,
department VARCHAR(128),
enabled TINYINT NOT NULL DEFAULT 1,
rule_level INT NOT NULL DEFAULT 7,
permission_version INT NOT NULL DEFAULT 1,
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_example_question (
id INT PRIMARY KEY AUTO_INCREMENT,
topic VARCHAR(32) NOT NULL,
question VARCHAR(255) NOT NULL,
description TEXT,
enabled TINYINT NOT NULL DEFAULT 1,
sort INT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_topic_sort (topic, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_question_candidate (
id INT PRIMARY KEY AUTO_INCREMENT,
topic VARCHAR(32) NOT NULL,
question VARCHAR(255) NOT NULL,
status VARCHAR(32) NOT NULL DEFAULT 'pending',
count INT NOT NULL DEFAULT 1,
last_seen_at INT NOT NULL,
create_time INT NOT NULL,
update_time INT NOT NULL,
INDEX idx_status_seen (status, last_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_data_source (
id INT PRIMARY KEY AUTO_INCREMENT,
source_type VARCHAR(32) NOT NULL UNIQUE,
name VARCHAR(64) NOT NULL,
enabled TINYINT NOT NULL DEFAULT 0,
status VARCHAR(32) NOT NULL DEFAULT 'closed',
latest_sync INT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_grid_import (
id INT PRIMARY KEY AUTO_INCREMENT,
month VARCHAR(16),
file_name VARCHAR(255) NOT NULL,
status VARCHAR(32) NOT NULL,
total_rows INT NOT NULL DEFAULT 0,
success_rows INT NOT NULL DEFAULT 0,
failed_rows INT NOT NULL DEFAULT 0,
operator VARCHAR(64),
create_time INT NOT NULL,
complete_time INT NOT NULL DEFAULT 0,
INDEX idx_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_grid_import_error (
id INT PRIMARY KEY AUTO_INCREMENT,
import_id INT NOT NULL,
row_index INT NOT NULL,
reason TEXT NOT NULL,
raw_data LONGTEXT,
INDEX idx_import_id (import_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_grid_import_audit (
	id INT PRIMARY KEY AUTO_INCREMENT,
	import_id INT NOT NULL,
	action VARCHAR(32) NOT NULL,
	operator VARCHAR(64) NOT NULL DEFAULT '',
	detail TEXT,
	create_time INT NOT NULL,
	INDEX idx_import_id (import_id),
	INDEX idx_create_time (create_time)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_knowledge_base (
code VARCHAR(64) PRIMARY KEY,
name VARCHAR(128) NOT NULL,
enabled TINYINT NOT NULL DEFAULT 1,
sort INT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_user_knowledge_permission (
id INT PRIMARY KEY AUTO_INCREMENT,
user_id INT NOT NULL,
knowledge_code VARCHAR(64) NOT NULL,
enabled TINYINT NOT NULL DEFAULT 0,
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_user_knowledge (user_id, knowledge_code),
INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_operation_log (
id INT PRIMARY KEY AUTO_INCREMENT,
log_type VARCHAR(16) NOT NULL,
username VARCHAR(64) NOT NULL,
action_type VARCHAR(64) NOT NULL,
content TEXT,
result VARCHAR(32) NOT NULL DEFAULT 'success',
create_time INT NOT NULL,
INDEX idx_log_type_time (log_type, create_time),
INDEX idx_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS case_list (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
responsibility_unit VARCHAR(100),
case_number VARCHAR(100),
case_source VARCHAR(100),
report_time DATETIME,
pending_step VARCHAR(50),
case_type VARCHAR(200),
region VARCHAR(200),
case_location VARCHAR(500),
description TEXT,
create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
update_time DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
INDEX idx_case_number (case_number),
INDEX idx_report_time (report_time),
INDEX idx_case_type (case_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS metric_catalog (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
topic VARCHAR(32) NOT NULL,
metric_name VARCHAR(64) NOT NULL,
display_name VARCHAR(128) NOT NULL,
description VARCHAR(512) DEFAULT '',
unit VARCHAR(32) DEFAULT '',
dimensions JSON DEFAULT NULL,
default_threshold DECIMAL(12,2) DEFAULT NULL,
threshold_direction VARCHAR(16) DEFAULT 'above',
related_fast_path INT DEFAULT NULL,
chart_type_hint VARCHAR(16) DEFAULT 'line',
is_active TINYINT(1) NOT NULL DEFAULT 1,
create_time INT NOT NULL,
update_time INT NOT NULL,
UNIQUE KEY uk_topic_metric (topic, metric_name),
INDEX idx_topic_active (topic, is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS topic_knowledge_binding (
	id INT PRIMARY KEY AUTO_INCREMENT,
	topic VARCHAR(32) NOT NULL,
	knowledge_code VARCHAR(64) NOT NULL,
	enabled TINYINT NOT NULL DEFAULT 1,
	create_time INT NOT NULL,
	update_time INT NOT NULL,
	UNIQUE KEY uk_topic_knowledge (topic, knowledge_code),
	INDEX idx_topic (topic)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
}

func SeedAdminTables(ctx context.Context, db gdb.DB) error {
	now := int(gtime.Timestamp())
	if count, err := db.Model("admin_account").Ctx(ctx).Count(); err == nil && count == 0 {
		verify := 8137
		password, err := genAdminPassword("admin123", verify)
		if err != nil {
			return err
		}
		if _, err = db.Model("admin_account").Ctx(ctx).Data(g.Map{
			"username":    "admin",
			"password":    password,
			"verify":      verify,
			"role":        "administrator",
			"enabled":     1,
			"create_time": now,
			"update_time": now,
		}).Insert(); err != nil {
			return err
		}
	}
	if count, err := db.Model("admin_example_question").Ctx(ctx).Count(); err == nil && count == 0 {
		seeds := []g.Map{
			{"topic": "grid", "question": "本月高新区案件结案率是多少？", "description": "查询网格案件办理成效", "enabled": 1, "sort": 10, "create_time": now, "update_time": now},
			{"topic": "population", "question": "过去一周人流进出趋势如何？", "description": "展示每日进出人数对比", "enabled": 1, "sort": 20, "create_time": now, "update_time": now},
			{"topic": "traffic", "question": "今日港澳车辆占比是多少？", "description": "统计重点卡口跨境车辆情况", "enabled": 1, "sort": 30, "create_time": now, "update_time": now},
		}
		for _, seed := range seeds {
			if _, err := db.Model("admin_example_question").Ctx(ctx).Data(seed).Insert(); err != nil {
				return err
			}
		}
	}
	if count, err := db.Model("admin_data_source").Ctx(ctx).Count(); err == nil && count == 0 {
		seeds := []g.Map{
			{"source_type": "population", "name": "人流数据接入", "enabled": 0, "status": "closed", "latest_sync": 0, "create_time": now, "update_time": now},
			{"source_type": "traffic", "name": "车流数据接入", "enabled": 0, "status": "closed", "latest_sync": 0, "create_time": now, "update_time": now},
			{"source_type": "grid", "name": "网格月度导入", "enabled": 1, "status": "ready", "latest_sync": 0, "create_time": now, "update_time": now},
		}
		for _, seed := range seeds {
			if _, err := db.Model("admin_data_source").Ctx(ctx).Data(seed).Insert(); err != nil {
				return err
			}
		}
	}
	seeds := []g.Map{
		{"code": "policy_files", "name": "政策文件库", "enabled": 1, "sort": 10, "create_time": now, "update_time": now},
		{"code": "laws", "name": "法规知识库", "enabled": 1, "sort": 20, "create_time": now, "update_time": now},
		{"code": "service_guides", "name": "办事指南库", "enabled": 1, "sort": 30, "create_time": now, "update_time": now},
		{"code": "case_library", "name": "历史案例库", "enabled": 1, "sort": 40, "create_time": now, "update_time": now},
	}
	for _, seed := range seeds {
		count, err := db.Model("admin_knowledge_base").Ctx(ctx).Where("code = ?", seed["code"]).Count()
		if err != nil {
			return err
		}
		if count == 0 {
			if _, err := db.Model("admin_knowledge_base").Ctx(ctx).Data(seed).Insert(); err != nil {
				return err
			}
		}
	}
	// Seed topic-knowledge bindings
	if count, err := db.Model("topic_knowledge_binding").Ctx(ctx).Count(); err == nil && count == 0 {
		bindings := []g.Map{
			{"topic": "grid", "knowledge_code": "policy", "enabled": 1, "create_time": now, "update_time": now},
			{"topic": "grid", "knowledge_code": "manual", "enabled": 1, "create_time": now, "update_time": now},
			{"topic": "grid", "knowledge_code": "form", "enabled": 1, "create_time": now, "update_time": now},
			{"topic": "grid", "knowledge_code": "case", "enabled": 1, "create_time": now, "update_time": now},
			{"topic": "traffic", "knowledge_code": "policy", "enabled": 1, "create_time": now, "update_time": now},
			{"topic": "traffic", "knowledge_code": "rule", "enabled": 1, "create_time": now, "update_time": now},
			{"topic": "population", "knowledge_code": "manual", "enabled": 1, "create_time": now, "update_time": now},
			{"topic": "population", "knowledge_code": "form", "enabled": 1, "create_time": now, "update_time": now},
		}
		for _, b := range bindings {
			if _, err := db.Model("topic_knowledge_binding").Ctx(ctx).Data(b).Insert(); err != nil {
				return err
			}
		}
	}
	return nil
}

func listAdminUsers(ctx context.Context, page, pageSize int) ([]v1.AdminUserItem, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize
	total, _ := g.DB("master").Model("user").Ctx(ctx).Count()
	records, err := g.DB("master").Model("user").Ctx(ctx).Fields("user_id, username, rule_level, last_login_tme, update_time").OrderAsc("user_id").Limit(pageSize).Offset(offset).All()
	if err != nil {
		return []v1.AdminUserItem{}, 0, err
	}
	list := make([]v1.AdminUserItem, 0, len(records))
	for _, record := range records {
		id := record["user_id"].Int()
		profile := getProfileMap(ctx, id)
		ruleLevel := record["rule_level"].Int()
		if profile["rule_level"].Val() != nil {
			ruleLevel = profile["rule_level"].Int()
		}
		enabled := true
		if profile["enabled"].Val() != nil {
			enabled = profile["enabled"].Int() != 0
		}
		displayName := profile["display_name"].String()
		if displayName == "" {
			displayName = record["username"].String()
		}
		lastLoginAt := maxInt(record["last_login_tme"].Int(), profile["last_login_at"].Int())
		list = append(list, v1.AdminUserItem{
			UserId:            id,
			Username:          record["username"].String(),
			DisplayName:       displayName,
			Department:        profile["department"].String(),
			Enabled:           enabled,
			RuleLevel:         ruleLevel,
			PermissionVersion: profile["permission_version"].Int(),
			Permissions:       permissionsFromRuleLevel(ruleLevel),
			QaPermissions:     knowledgePermissionsForUser(ctx, id),
			LastLoginAt:       lastLoginAt,
			UpdateTime:        maxInt(record["update_time"].Int(), profile["update_time"].Int()),
		})
	}
	return list, total, nil
}

func getAdminUser(ctx context.Context, id int) (v1.AdminUserItem, error) {
	users, _, err := listAdminUsers(ctx, 1, 500)
	if err != nil {
		return v1.AdminUserItem{}, err
	}
	for _, user := range users {
		if user.UserId == id {
			return user, nil
		}
	}
	return v1.AdminUserItem{UserId: id, Enabled: true, RuleLevel: 7, PermissionVersion: 1, Permissions: permissionsFromRuleLevel(7)}, nil
}

func upsertUserProfile(ctx context.Context, userId int, data g.Map) error {
	now := int(gtime.Timestamp())
	count, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).Data(data).Update()
		return err
	}
	insertData := g.Map{"user_id": userId, "department": "", "enabled": 1, "rule_level": 7, "permission_version": 1, "create_time": now, "update_time": now}
	for key, value := range data {
		insertData[key] = value
	}
	_, err = g.DB("master").Model("admin_user_profile").Ctx(ctx).Data(insertData).Insert()
	return err
}

func getProfileMap(ctx context.Context, userId int) gdb.Record {
	record, err := g.DB("master").Model("admin_user_profile").Ctx(ctx).Where("user_id = ?", userId).One()
	if err != nil || record == nil {
		return gdb.Record{}
	}
	return record
}

func bumpUserPermissionVersion(ctx context.Context, userId int) error {
	if userId <= 0 {
		return nil
	}
	now := int(gtime.Timestamp())
	record := getProfileMap(ctx, userId)
	version := record["permission_version"].Int()
	if version <= 0 {
		version = 1
	}
	return upsertUserProfile(ctx, userId, g.Map{"permission_version": version + 1, "update_time": now})
}

func clearUserPermissionCache(ctx context.Context, userId int) {
	if userId <= 0 {
		return
	}
	_, _ = consts.Cache.Remove(ctx, fmt.Sprintf("user_permission:%d", userId))
	_, _ = consts.Cache.Remove(ctx, fmt.Sprintf("user_bootstrap:%d", userId))
}

func ensureDefaultKnowledgePermissions(ctx context.Context, userId int, ruleLevel int, enabled bool) error {
	if userId <= 0 {
		return nil
	}
	count, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Where("user_id = ?", userId).Count()
	if err != nil || count > 0 {
		return err
	}
	bases, err := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Fields("code").Where("enabled = ?", 1).OrderAsc("sort").All()
	if err != nil {
		return err
	}
	now := int(gtime.Timestamp())
	for _, base := range bases {
		code := base["code"].String()
		allowed := enabled && (code == "policy" || code == "form" || (ruleLevel&7 == 7 && (code == "manual" || code == "rule" || code == "case")))
		if _, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Data(g.Map{
			"user_id":        userId,
			"knowledge_code": code,
			"enabled":        boolToInt(allowed),
			"create_time":    now,
			"update_time":    now,
		}).Insert(); err != nil {
			return err
		}
	}
	return nil
}

func knowledgePermissionsForUser(ctx context.Context, userId int) []v1.KnowledgePermission {
	bases, err := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Where("enabled = ?", 1).OrderAsc("sort").All()
	if err != nil {
		return []v1.KnowledgePermission{}
	}
	perms, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Where("user_id = ?", userId).All()
	if err != nil {
		consts.Logger.Warningf(ctx, "query user knowledge permissions failed: %v", err)
		return []v1.KnowledgePermission{}
	}
	enabledByCode := make(map[string]bool, len(perms))
	for _, perm := range perms {
		enabledByCode[normalizeQaKnowledgeCode(perm["knowledge_code"].String())] = perm["enabled"].Int() != 0
	}
	list := make([]v1.KnowledgePermission, 0, len(bases))
	for _, base := range bases {
		code := base["code"].String()
		list = append(list, v1.KnowledgePermission{
			Code:    code,
			Name:    base["name"].String(),
			Enabled: enabledByCode[code],
		})
	}
	return list
}

func normalizeQaKnowledgeCode(code string) string {
	switch strings.TrimSpace(code) {
	case "policy_files":
		return "policy"
	case "laws":
		return "manual"
	default:
		return strings.TrimSpace(code)
	}
}

func knowledgeBaseOptions(ctx context.Context) []v1.KnowledgePermission {
	bases, err := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Where("enabled = ?", 1).OrderAsc("sort").All()
	if err != nil {
		return []v1.KnowledgePermission{}
	}
	list := make([]v1.KnowledgePermission, 0, len(bases))
	for _, base := range bases {
		list = append(list, v1.KnowledgePermission{
			Code:    base["code"].String(),
			Name:    base["name"].String(),
			Enabled: true,
			DocType: base["doc_type"].String(),
		})
	}
	return list
}

func replaceUserKnowledgePermissions(ctx context.Context, userId int, permissions []v1.KnowledgePermission) error {
	now := int(gtime.Timestamp())
	if _, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Where("user_id = ?", userId).Delete(); err != nil {
		return err
	}
	for _, item := range permissions {
		if item.Code == "" {
			continue
		}
		if _, err := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Data(g.Map{
			"user_id":        userId,
			"knowledge_code": item.Code,
			"enabled":        boolToInt(item.Enabled),
			"create_time":    now,
			"update_time":    now,
		}).Insert(); err != nil {
			return err
		}
	}
	return nil
}

func getExampleQuestion(ctx context.Context, id int) (v1.ExampleQuestionItem, error) {
	record, err := g.DB("master").Model("admin_example_question").Ctx(ctx).Where("id = ?", id).One()
	if err != nil {
		return v1.ExampleQuestionItem{}, err
	}
	if record == nil {
		return v1.ExampleQuestionItem{}, gerror.New("question does not exist")
	}
	return exampleQuestionFromRecord(record), nil
}

func getDataSource(ctx context.Context, sourceType string) (v1.DataSourceItem, error) {
	record, err := g.DB("master").Model("admin_data_source").Ctx(ctx).Where("source_type = ?", sourceType).One()
	if err != nil {
		return v1.DataSourceItem{}, err
	}
	if record == nil {
		return v1.DataSourceItem{}, gerror.New("数据源不存在")
	}
	return dataSourceFromRecord(record), nil
}

func getGridImport(ctx context.Context, id int) (v1.GridImportItem, error) {
	record, err := g.DB("master").Model("admin_grid_import").Ctx(ctx).Where("id = ?", id).One()
	if err != nil {
		return v1.GridImportItem{}, err
	}
	if record == nil {
		return v1.GridImportItem{}, gerror.New("import record does not exist")
	}
	return gridImportFromRecord(record), nil
}

func updateQuestionCandidateStatus(ctx context.Context, id int, status string) (v1.QuestionCandidateItem, error) {
	_, err := g.DB("master").Model("admin_question_candidate").Ctx(ctx).Where("id = ?", id).Data(g.Map{
		"status":      status,
		"update_time": int(gtime.Timestamp()),
	}).Update()
	if err != nil {
		return v1.QuestionCandidateItem{}, err
	}
	record, err := g.DB("master").Model("admin_question_candidate").Ctx(ctx).Where("id = ?", id).One()
	if err != nil {
		return v1.QuestionCandidateItem{}, err
	}
	if record == nil {
		return v1.QuestionCandidateItem{}, gerror.New("候选问题不存在")
	}
	return questionCandidateFromRecord(record), nil
}

func scanExampleQuestions(records gdb.Result) []v1.ExampleQuestionItem {
	list := make([]v1.ExampleQuestionItem, 0, len(records))
	for _, record := range records {
		list = append(list, exampleQuestionFromRecord(record))
	}
	return list
}

func scanQuestionCandidates(records gdb.Result) []v1.QuestionCandidateItem {
	list := make([]v1.QuestionCandidateItem, 0, len(records))
	for _, record := range records {
		list = append(list, questionCandidateFromRecord(record))
	}
	return list
}

func scanDataSources(records gdb.Result) []v1.DataSourceItem {
	list := make([]v1.DataSourceItem, 0, len(records))
	for _, record := range records {
		list = append(list, dataSourceFromRecord(record))
	}
	return list
}

func scanGridImports(records gdb.Result) []v1.GridImportItem {
	list := make([]v1.GridImportItem, 0, len(records))
	for _, record := range records {
		list = append(list, gridImportFromRecord(record))
	}
	return list
}

func scanGridImportErrors(records gdb.Result) []v1.GridImportErrorItem {
	list := make([]v1.GridImportErrorItem, 0, len(records))
	for _, record := range records {
		list = append(list, v1.GridImportErrorItem{
			Id:       record["id"].Int(),
			ImportId: record["import_id"].Int(),
			RowIndex: record["row_index"].Int(),
			Reason:   record["reason"].String(),
			RawData:  record["raw_data"].String(),
		})
	}
	return list
}

func scanAdminLogs(records gdb.Result) []v1.AdminLogItem {
	list := make([]v1.AdminLogItem, 0, len(records))
	for _, record := range records {
		list = append(list, v1.AdminLogItem{
			Id:         record["id"].Int(),
			LogType:    record["log_type"].String(),
			Username:   record["username"].String(),
			ActionType: record["action_type"].String(),
			Content:    record["content"].String(),
			Result:     record["result"].String(),
			CreateTime: record["create_time"].Int(),
		})
	}
	return list
}

func insertAdminLog(ctx context.Context, logType string, username string, actionType string, content string, result string) error {
	if logType == "" {
		logType = "admin"
	}
	if username == "" {
		username = "admin"
	}
	if result == "" {
		result = "success"
	}
	_, err := g.DB("master").Model("admin_operation_log").Ctx(ctx).Data(g.Map{
		"log_type":    logType,
		"username":    username,
		"action_type": actionType,
		"content":     content,
		"result":      result,
		"create_time": int(gtime.Timestamp()),
	}).Insert()
	if err != nil {
		consts.Logger.Warningf(ctx, "insert admin audit log failed: %v", err)
	}
	return nil
}

func exampleQuestionFromRecord(record gdb.Record) v1.ExampleQuestionItem {
	return v1.ExampleQuestionItem{
		Id:          record["id"].Int(),
		Topic:       record["topic"].String(),
		Question:    record["question"].String(),
		Description: record["description"].String(),
		Enabled:     record["enabled"].Int() != 0,
		Sort:        record["sort"].Int(),
		UpdateTime:  record["update_time"].Int(),
	}
}

func questionCandidateFromRecord(record gdb.Record) v1.QuestionCandidateItem {
	return v1.QuestionCandidateItem{
		Id:         record["id"].Int(),
		Topic:      record["topic"].String(),
		Question:   record["question"].String(),
		Status:     record["status"].String(),
		Count:      record["count"].Int(),
		LastSeenAt: record["last_seen_at"].Int(),
		UpdateTime: record["update_time"].Int(),
	}
}

func dataSourceFromRecord(record gdb.Record) v1.DataSourceItem {
	return v1.DataSourceItem{
		Type:       record["source_type"].String(),
		Name:       record["name"].String(),
		Enabled:    record["enabled"].Int() != 0,
		Status:     record["status"].String(),
		LatestSync: record["latest_sync"].Int(),
		UpdateTime: record["update_time"].Int(),
	}
}

func gridImportFromRecord(record gdb.Record) v1.GridImportItem {
	return v1.GridImportItem{
		Id:           record["id"].Int(),
		Month:        record["month"].String(),
		FileName:     record["file_name"].String(),
		Status:       record["status"].String(),
		TotalRows:    record["total_rows"].Int(),
		SuccessRows:  record["success_rows"].Int(),
		FailedRows:   record["failed_rows"].Int(),
		Operator:     record["operator"].String(),
		CreateTime:   record["create_time"].Int(),
		CompleteTime: record["complete_time"].Int(),
	}
}

func executeAdminAidgpSync(ctx context.Context, syncType string, knowledgeCode string) (*v1.AdminSyncRes, error) {
	// Check if the data source is enabled before running sync
	sourceType := syncTypeToSourceType(syncType)
	if sourceType != "" {
		record, _ := g.DB("master").Model("admin_data_source").Ctx(ctx).Where("source_type = ?", sourceType).One()
		if record != nil && record["enabled"].Int() == 0 {
			return nil, gerror.Newf("数据源 %s 已关闭，请先启用后再同步", sourceType)
		}
	}

	provider := adminSyncProvider()
	now := int(gtime.Timestamp())
	taskId, err := insertAdminSyncTask(ctx, provider, syncType, "running", "同步任务已开始")
	if err != nil {
		return nil, err
	}
	client := adminAidgpClient(provider)
	result, syncErr := callAdminAidgpSync(ctx, client, syncType, knowledgeCode)
	status := "success"
	message := result.Message
	if syncErr != nil {
		status = "failed"
		message = syncErr.Error()
		result.FailureCount++
		result.Logs = append(result.Logs, aidgp.SyncLog{Action: "execute", Status: "failed", Message: syncErr.Error()})
		// Generate alert for sync failure
		alertTopic := sourceType
		if alertTopic == "" {
			alertTopic = "system"
		}
		service.Alert().AddAlert(ctx, model.AlertItem{
			Topic:      alertTopic,
			Title:      fmt.Sprintf("%s数据同步失败", sourceType),
			Content:    fmt.Sprintf("AIDGP %s 同步失败: %s", syncType, syncErr.Error()),
			Question:   "",
			Level:      "warning",
			CreateTime: int(gtime.Timestamp()),
		})
	} else if result.FailureCount > 0 {
		status = "partial_failed"
	} else if result.SuccessCount == 0 && result.SkippedCount > 0 {
		status = "skipped"
	}
	if syncType == aidgp.SyncKnowledgeBases && syncErr == nil {
		disableOrphanedKnowledgeBases(ctx, result)
	}
	if err = insertAdminSyncLogs(ctx, taskId, provider, syncType, result.Logs); err != nil {
		return nil, err
	}
	finishedAt := int(gtime.Timestamp())
	if err = finishAdminSyncTask(ctx, taskId, status, message, result.SuccessCount, result.FailureCount, result.SkippedCount, finishedAt); err != nil {
		return nil, err
	}
	// Update admin_data_source latest_sync and status
	if sourceType != "" {
		dsStatus := "ready"
		if status == "failed" || status == "partial_failed" {
			dsStatus = "error"
		}
		if _, dsErr := g.DB("master").Model("admin_data_source").Ctx(ctx).
			Where("source_type = ?", sourceType).
			Data(g.Map{
				"latest_sync": finishedAt,
				"status":      dsStatus,
				"update_time": finishedAt,
			}).Update(); dsErr != nil {
			consts.Logger.Warningf(ctx, "更新数据源状态失败: %s", dsErr.Error())
		}
	}
	return &v1.AdminSyncRes{
		TaskId:       taskId,
		Provider:     provider,
		SyncType:     syncType,
		Status:       status,
		Message:      message,
		SuccessCount: result.SuccessCount,
		FailureCount: result.FailureCount,
		SkippedCount: result.SkippedCount,
		StartedAt:    now,
		FinishedAt:   finishedAt,
		CreateTime:   now,
		UpdateTime:   finishedAt,
	}, nil
}

func executeAdminAidgpSyncWithScope(ctx context.Context, syncType string, knowledgeCode string) (*v1.AdminSyncRes, error) {
	sourceType := syncTypeToSourceType(syncType)
	if sourceType != "" {
		record, _ := g.DB("master").Model("admin_data_source").Ctx(ctx).Where("source_type = ?", sourceType).One()
		if record != nil && record["enabled"].Int() == 0 {
			return nil, gerror.Newf("数据源 %s 已关闭，请先启用后再同步", sourceType)
		}
	}

	provider := adminSyncProvider()
	now := int(gtime.Timestamp())
	taskId, err := insertAdminSyncTask(ctx, provider, syncType, "running", "增量同步任务已开始")
	if err != nil {
		return nil, err
	}
	client := adminAidgpClient(provider)

	scope := aidgp.SyncScope{KnowledgeCode: knowledgeCode}
	if sourceType != "" {
		dsRecord, _ := g.DB("master").Model("admin_data_source").Ctx(ctx).
			Where("source_type = ?", sourceType).One()
		if dsRecord != nil && dsRecord["latest_sync"].Int() > 0 {
			scope.Since = time.Unix(int64(dsRecord["latest_sync"].Int()), 0).Format(time.RFC3339)
		}
	}

	result, syncErr := callAdminAidgpSyncWithScope(ctx, client, syncType, scope)
	status := "success"
	message := result.Message
	if syncErr != nil {
		status = "failed"
		message = syncErr.Error()
		result.FailureCount++
		result.Logs = append(result.Logs, aidgp.SyncLog{Action: "execute", Status: "failed", Message: syncErr.Error()})
		alertTopic := sourceType
		if alertTopic == "" {
			alertTopic = "system"
		}
		service.Alert().AddAlert(ctx, model.AlertItem{
			Topic:      alertTopic,
			Title:      fmt.Sprintf("%s数据同步失败", sourceType),
			Content:    fmt.Sprintf("AIDGP %s 同步失败: %s", syncType, syncErr.Error()),
			Level:      "warning",
			CreateTime: int(gtime.Timestamp()),
		})
	} else if result.FailureCount > 0 {
		status = "partial_failed"
	} else if result.SuccessCount == 0 && result.SkippedCount > 0 {
		status = "skipped"
	}
	if syncType == aidgp.SyncKnowledgeBases && syncErr == nil {
		disableOrphanedKnowledgeBases(ctx, result)
	}
	if err = insertAdminSyncLogs(ctx, taskId, provider, syncType, result.Logs); err != nil {
		return nil, err
	}
	finishedAt := int(gtime.Timestamp())
	if err = finishAdminSyncTask(ctx, taskId, status, message, result.SuccessCount, result.FailureCount, result.SkippedCount, finishedAt); err != nil {
		return nil, err
	}
	if sourceType != "" {
		dsStatus := "ready"
		if status == "failed" || status == "partial_failed" {
			dsStatus = "error"
		}
		if _, dsErr := g.DB("master").Model("admin_data_source").Ctx(ctx).
			Where("source_type = ?", sourceType).
			Data(g.Map{
				"latest_sync": finishedAt,
				"status":      dsStatus,
				"update_time": finishedAt,
			}).Update(); dsErr != nil {
			consts.Logger.Warningf(ctx, "更新数据源状态失败: %s", dsErr.Error())
		}
	}
	return &v1.AdminSyncRes{
		TaskId:       taskId,
		Provider:     provider,
		SyncType:     syncType,
		Status:       status,
		Message:      message,
		SuccessCount: result.SuccessCount,
		FailureCount: result.FailureCount,
		SkippedCount: result.SkippedCount,
		StartedAt:    now,
		FinishedAt:   finishedAt,
		CreateTime:   now,
		UpdateTime:   finishedAt,
	}, nil
}

func adminSyncProvider() string {
	if consts.GetConfig() == nil || consts.GetConfig().QaConfig == nil || consts.GetConfig().QaConfig.Sync == nil {
		return aidgp.ProviderMock
	}
	provider := strings.TrimSpace(consts.GetConfig().QaConfig.Sync.Provider)
	if provider == "" {
		return aidgp.ProviderMock
	}
	return provider
}

func adminAidgpClient(provider string) aidgp.Client {
	cfg := aidgp.Config{Provider: provider}
	if consts.GetConfig() != nil && consts.GetConfig().QaConfig != nil && consts.GetConfig().QaConfig.Sync != nil && consts.GetConfig().QaConfig.Sync.Aidgp != nil {
		aidgpCfg := consts.GetConfig().QaConfig.Sync.Aidgp
		cfg.BaseUrl = aidgpCfg.BaseUrl
		cfg.AppKey = aidgpCfg.AppKey
		cfg.AppSecret = aidgpCfg.AppSecret
		cfg.TokenPath = aidgpCfg.TokenPath
		cfg.TrafficQueryPath = aidgpCfg.TrafficQueryPath
		cfg.PopulationQueryPath = aidgpCfg.PopulationQueryPath
		cfg.GridQueryPath = aidgpCfg.GridQueryPath
		cfg.KnowledgeBasesPath = aidgpCfg.KnowledgeBasesPath
		cfg.DocumentsPath = aidgpCfg.DocumentsPath
		cfg.DocumentSegmentsPath = aidgpCfg.DocumentSegmentsPath
		cfg.KnowledgePermissionsPath = aidgpCfg.KnowledgePermissionsPath
		cfg.TimeoutSeconds = aidgpCfg.TimeoutSeconds
		cfg.RetryTimes = aidgpCfg.RetryTimes
		cfg.TokenExpireSkewSeconds = aidgpCfg.TokenExpireSkewSeconds
	}
	return aidgp.NewClient(cfg)
}

func syncTypeToSourceType(syncType string) string {
	switch syncType {
	case aidgp.SyncTrafficData:
		return "traffic"
	case aidgp.SyncPopulationData:
		return "population"
	case aidgp.SyncGridData:
		return "grid"
	default:
		return ""
	}
}

func callAdminAidgpSync(ctx context.Context, client aidgp.Client, syncType string, knowledgeCode string) (aidgp.SyncResult, error) {
	scope := aidgp.SyncScope{KnowledgeCode: knowledgeCode}
	return callAdminAidgpSyncWithScope(ctx, client, syncType, scope)
}

func callAdminAidgpSyncWithScope(ctx context.Context, client aidgp.Client, syncType string, scope aidgp.SyncScope) (aidgp.SyncResult, error) {
	switch syncType {
	case aidgp.SyncKnowledgeBases:
		return client.SyncKnowledgeBases(ctx, scope)
	case aidgp.SyncDocuments:
		return client.SyncDocuments(ctx, scope)
	case aidgp.SyncGridData:
		return client.SyncGridData(ctx, scope)
	case aidgp.SyncTrafficData:
		return client.SyncTrafficData(ctx, scope)
	case aidgp.SyncPopulationData:
		return client.SyncPopulationData(ctx, scope)
	default:
		return aidgp.SyncResult{}, fmt.Errorf("unsupported sync type: %s", syncType)
	}
}

func insertAdminSyncTask(ctx context.Context, provider string, syncType string, status string, message string) (int64, error) {
	now := int(gtime.Timestamp())
	return g.DB("master").Model("qa_sync_task").Ctx(ctx).Data(g.Map{
		"provider":      provider,
		"sync_type":     syncType,
		"status":        status,
		"message":       message,
		"success_count": 0,
		"failure_count": 0,
		"skipped_count": 0,
		"started_at":    now,
		"finished_at":   0,
		"create_time":   now,
		"update_time":   now,
	}).InsertAndGetId()
}

func finishAdminSyncTask(ctx context.Context, taskId int64, status string, message string, successCount int, failureCount int, skippedCount int, finishedAt int) error {
	_, err := g.DB("master").Model("qa_sync_task").Ctx(ctx).
		Where("id = ?", taskId).
		Data(g.Map{
			"status":        status,
			"message":       message,
			"success_count": successCount,
			"failure_count": failureCount,
			"skipped_count": skippedCount,
			"finished_at":   finishedAt,
			"update_time":   finishedAt,
		}).
		Update()
	return err
}

func insertAdminSyncLogs(ctx context.Context, taskId int64, provider string, syncType string, logs []aidgp.SyncLog) error {
	now := int(gtime.Timestamp())
	if len(logs) == 0 {
		logs = []aidgp.SyncLog{{Action: "execute", Status: "success", Message: "同步执行完成，无明细记录"}}
	}
	for _, item := range logs {
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "success"
		}
		if _, err := g.DB("master").Model("qa_sync_log").Ctx(ctx).Data(g.Map{
			"task_id":     taskId,
			"provider":    provider,
			"sync_type":   syncType,
			"external_id": item.ExternalId,
			"local_id":    item.LocalId,
			"action":      item.Action,
			"status":      status,
			"message":     item.Message,
			"create_time": now,
		}).Insert(); err != nil {
			return err
		}
	}
	return nil
}

func scanAdminSyncTasks(records gdb.Result) []v1.AdminSyncTaskItem {
	list := make([]v1.AdminSyncTaskItem, 0, len(records))
	for _, record := range records {
		list = append(list, v1.AdminSyncTaskItem{
			TaskId:       record["id"].Int64(),
			Provider:     record["provider"].String(),
			SyncType:     record["sync_type"].String(),
			Status:       record["status"].String(),
			Message:      record["message"].String(),
			SuccessCount: record["success_count"].Int(),
			FailureCount: record["failure_count"].Int(),
			SkippedCount: record["skipped_count"].Int(),
			StartedAt:    record["started_at"].Int(),
			FinishedAt:   record["finished_at"].Int(),
			CreateTime:   record["create_time"].Int(),
			UpdateTime:   record["update_time"].Int(),
		})
	}
	return list
}

func scanAdminSyncLogs(records gdb.Result) []v1.AdminSyncLogItem {
	list := make([]v1.AdminSyncLogItem, 0, len(records))
	for _, record := range records {
		list = append(list, v1.AdminSyncLogItem{
			LogId:      record["id"].Int64(),
			TaskId:     record["task_id"].Int64(),
			Provider:   record["provider"].String(),
			SyncType:   record["sync_type"].String(),
			ExternalId: record["external_id"].String(),
			LocalId:    record["local_id"].String(),
			Action:     record["action"].String(),
			Status:     record["status"].String(),
			Message:    record["message"].String(),
			CreateTime: record["create_time"].Int(),
		})
	}
	return list
}

func reconcileSyncType(ctx context.Context, syncType string, localTable string, localWhere string) v1.AdminSyncReconcileItem {
	rawRecord, _ := g.DB("master").Model("aidgp_sync_record").Ctx(ctx).
		Fields("COUNT(*) AS raw_count, MAX(last_sync_time) AS latest_raw_at").
		Where("sync_type = ?", syncType).
		One()
	rawCount := 0
	latestRawAt := 0
	if rawRecord != nil {
		rawCount = rawRecord["raw_count"].Int()
		latestRawAt = rawRecord["latest_raw_at"].Int()
	}
	localModel := g.DB("master").Model(localTable).Ctx(ctx)
	if strings.TrimSpace(localWhere) != "" {
		localModel = localModel.Where(localWhere)
	}
	localCount, err := localModel.Count()
	status := "ok"
	if err != nil {
		status = "local_error"
		localCount = 0
	} else if rawCount == 0 && localCount == 0 {
		status = "empty"
	} else if rawCount != localCount {
		status = "diff"
	}
	return v1.AdminSyncReconcileItem{
		SyncType:    syncType,
		RawCount:    rawCount,
		LocalCount:  localCount,
		Difference:  localCount - rawCount,
		LatestRawAt: latestRawAt,
		Status:      status,
	}
}

type gridParseError struct {
	RowIndex int
	Reason   string
	RawData  string
}

type gridCaseRecord struct {
	ResponsibilityUnit string
	CaseNumber         string
	CaseSource         string
	ReportTime         string
	PendingStep        string
	CaseType           string
	Region             string
	CaseLocation       string
	Description        string
}

func (r gridCaseRecord) toMap() g.Map {
	reportTime := interface{}(nil)
	if r.ReportTime != "" {
		reportTime = r.ReportTime
	}
	return g.Map{
		"responsibility_unit": r.ResponsibilityUnit,
		"case_number":         r.CaseNumber,
		"case_source":         r.CaseSource,
		"report_time":         reportTime,
		"pending_step":        r.PendingStep,
		"case_type":           r.CaseType,
		"region":              r.Region,
		"case_location":       r.CaseLocation,
		"description":         r.Description,
	}
}

func parseGridImportFile(fileName string, content []byte) ([]gridCaseRecord, []gridParseError, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	var allRows [][]string
	var err error
	switch ext {
	case ".xlsx":
		allRows, err = readXLSXRows(content)
	case ".xls":
		allRows, err = readXLSRows(content)
	case ".csv":
		allRows, err = readCSVRows(content)
	default:
		return nil, nil, gerror.New("only .xlsx, .xls and .csv files are supported")
	}
	if err != nil {
		return nil, nil, err
	}
	return parseGridCaseRows(allRows)
}

func readXLSXRows(content []byte) ([][]string, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, gerror.Newf("failed to parse Excel file: %v", err)
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return nil, gerror.New("Excel file has no sheet")
	}
	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		return nil, gerror.Newf("failed to read rows: %v", err)
	}
	return rows, nil
}

func readXLSRows(content []byte) ([][]string, error) {
	// 中文 Excel 通常使用 GBK 编码
	workbook, err := xls.OpenReader(bytes.NewReader(content), "gbk")
	if err != nil {
		// 如果 GBK 失败，尝试 UTF-8
		workbook, err = xls.OpenReader(bytes.NewReader(content), "utf-8")
		if err != nil {
			return nil, gerror.Newf("failed to parse .xls file: %v", err)
		}
	}
	sheets := workbook.NumSheets()
	if sheets == 0 {
		return nil, gerror.New(".xls file has no sheet")
	}
	sheet := workbook.GetSheet(0)
	if sheet == nil {
		return nil, gerror.New("failed to get first sheet")
	}
	rows := make([][]string, 0)
	for i := 0; i < int(sheet.MaxRow); i++ {
		row := sheet.Row(i)
		if row == nil {
			continue
		}
		cells := make([]string, 0)
		for j := 0; j < row.LastCol(); j++ {
			cell := row.Col(j)
			cells = append(cells, cell)
		}
		rows = append(rows, cells)
	}
	return rows, nil
}

func readCSVRows(content []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, gerror.New("failed to parse CSV file")
	}
	return rows, nil
}

func parseGridCaseRows(rows [][]string) ([]gridCaseRecord, []gridParseError, error) {
	if len(rows) <= 1 {
		return nil, []gridParseError{{RowIndex: 1, Reason: "file has no data rows", RawData: ""}}, nil
	}

	headerMap := buildGridHeaderMap(rows[0])
	missingHeaders := requiredGridHeadersMissing(headerMap)
	if len(missingHeaders) > 0 {
		return nil, nil, gerror.Newf("template columns missing: %s", strings.Join(missingHeaders, ", "))
	}

	cases := make([]gridCaseRecord, 0)
	errors := make([]gridParseError, 0)
	for index, row := range rows {
		if index == 0 || rowIsBlank(row) {
			continue
		}
		item, reason := buildGridCaseRecord(headerMap, row)
		if reason != "" {
			errors = append(errors, gridParseError{
				RowIndex: index + 1,
				Reason:   reason,
				RawData:  rowToJSONWithHeaders(rows[0], row),
			})
			continue
		}
		cases = append(cases, item)
	}
	return cases, errors, nil
}

func buildGridHeaderMap(header []string) map[string]int {
	result := make(map[string]int)
	for index, value := range header {
		key := normalizeGridHeader(value)
		if key != "" {
			result[key] = index
		}
	}
	return result
}

func requiredGridHeadersMissing(headerMap map[string]int) []string {
	required := []string{"责任单位", "案件编号", "案件来源", "上报时间", "待办环节", "案件类别", "所属区域", "案件位置", "问题描述"}
	missing := make([]string, 0)
	for _, header := range required {
		if _, ok := headerMap[normalizeGridHeader(header)]; !ok {
			missing = append(missing, header)
		}
	}
	return missing
}

func buildGridCaseRecord(headerMap map[string]int, row []string) (gridCaseRecord, string) {
	item := gridCaseRecord{
		ResponsibilityUnit: getGridCell(headerMap, row, "责任单位"),
		CaseNumber:         getGridCell(headerMap, row, "案件编号"),
		CaseSource:         getGridCell(headerMap, row, "案件来源"),
		PendingStep:        getGridCell(headerMap, row, "待办环节"),
		CaseType:           getGridCell(headerMap, row, "案件类别"),
		Region:             getGridCell(headerMap, row, "所属区域"),
		CaseLocation:       getGridCell(headerMap, row, "案件位置"),
		Description:        getGridCell(headerMap, row, "问题描述"),
	}
	if item.CaseNumber == "" {
		return item, "case_number is required"
	}
	reportTime := getGridCell(headerMap, row, "上报时间")
	parsedTime, err := normalizeCaseReportTime(reportTime)
	if reportTime != "" && err != nil {
		return item, "invalid report_time"
	}
	item.ReportTime = parsedTime
	return item, ""
}

func getGridCell(headerMap map[string]int, row []string, header string) string {
	index, ok := headerMap[normalizeGridHeader(header)]
	if !ok || index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func normalizeGridHeader(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\t", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

func normalizeCaseReportTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if serial, err := strconv.ParseFloat(value, 64); err == nil && serial > 20000 && serial < 100000 {
		if t, err := excelize.ExcelDateToTime(serial, false); err == nil {
			return t.Format("2006-01-02 15:04:05"), nil
		}
	}
	layouts := []string{
		"2006-01-02 15:04:05", "2006/01/02 15:04:05", "2006-1-2 15:04:05", "2006/1/2 15:04:05",
		"2006-01-02 15:04", "2006/01/02 15:04", "2006-1-2 15:04", "2006/1/2 15:04",
		"2006-01-02", "2006/01/02", "2006-1-2", "2006/1/2",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t.Format("2006-01-02 15:04:05"), nil
		}
	}
	return "", fmt.Errorf("unsupported report_time: %s", value)
}

func rowIsBlank(row []string) bool {
	return nonEmptyCellCount(row) == 0
}

func nonEmptyCellCount(row []string) int {
	count := 0
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			count++
		}
	}
	return count
}

func rowToJSONWithHeaders(headers []string, row []string) string {
	data := make(map[string]string)
	for index, header := range headers {
		key := strings.TrimSpace(header)
		if key == "" {
			key = fmt.Sprintf("column_%d", index+1)
		}
		if index < len(row) {
			data[key] = strings.TrimSpace(row[index])
		} else {
			data[key] = ""
		}
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return strings.Join(row, ",")
	}
	return string(bytes)
}
func permissionsFromRuleLevel(ruleLevel int) []v1.TopicPermission {
	return []v1.TopicPermission{
		{Topic: "grid", Enabled: ruleLevel&1 != 0},
		{Topic: "population", Enabled: ruleLevel&2 != 0},
		{Topic: "traffic", Enabled: ruleLevel&4 != 0},
	}
}

func ruleLevelFromPermissions(permissions []v1.TopicPermission) int {
	ruleLevel := 0
	for _, permission := range permissions {
		if !permission.Enabled {
			continue
		}
		switch permission.Topic {
		case "grid":
			ruleLevel |= 1
		case "population":
			ruleLevel |= 2
		case "traffic":
			ruleLevel |= 4
		}
	}
	return ruleLevel
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func genAdminPassword(password string, code int) (string, error) {
	return gmd5.Encrypt(fmt.Sprintf("chatdb-admin:%s:%d", password, code))
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

// AdminCaseList 查询案件列表
func (c *ControllerV1) AdminCaseList(ctx context.Context, req *v1.AdminCaseListReq) (res *v1.AdminCaseListRes, err error) {

	db := g.DB("master").Model("case_list")

	// 构建查询条件
	if req.CaseNumber != "" {
		db = db.WhereLike("case_number", "%"+utility.EscapeLike(req.CaseNumber)+"%")
	}
	if req.CaseType != "" {
		db = db.WhereLike("case_type", "%"+utility.EscapeLike(req.CaseType)+"%")
	}
	if req.Region != "" {
		db = db.WhereLike("region", "%"+utility.EscapeLike(req.Region)+"%")
	}
	if req.StartDate != "" {
		db = db.WhereGTE("report_time", req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		db = db.WhereLTE("report_time", req.EndDate+" 23:59:59")
	}

	// 获取总数
	total, err := db.Count()
	if err != nil {
		return nil, err
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	records, err := db.OrderDesc("id").Limit(offset, req.PageSize).All()
	if err != nil {
		return nil, err
	}

	list := make([]v1.CaseListItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.CaseListItem{
			Id:                 r["id"].Int(),
			ResponsibilityUnit: r["responsibility_unit"].String(),
			CaseNumber:         r["case_number"].String(),
			CaseSource:         r["case_source"].String(),
			ReportTime:         r["report_time"].String(),
			PendingStep:        r["pending_step"].String(),
			CaseType:           r["case_type"].String(),
			Region:             r["region"].String(),
			CaseLocation:       r["case_location"].String(),
			Description:        r["description"].String(),
			CreateTime:         r["create_time"].String(),
			UpdateTime:         r["update_time"].String(),
		})
	}

	return &v1.AdminCaseListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// AdminCaseDelete 删除单个案件
func (c *ControllerV1) AdminCaseDelete(ctx context.Context, req *v1.AdminCaseDeleteReq) (res *v1.AdminCaseDeleteRes, err error) {

	_, err = g.DB("master").Model("case_list").Ctx(ctx).Where("id = ?", req.Id).Delete()
	if err != nil {
		return nil, err
	}

	return &v1.AdminCaseDeleteRes{}, nil
}

// AdminCaseDeleteAll 删除所有案件
func (c *ControllerV1) AdminCaseDeleteAll(ctx context.Context, req *v1.AdminCaseDeleteAllReq) (res *v1.AdminCaseDeleteAllRes, err error) {
	if !req.Confirm {
		return nil, gerror.New("必须传递 confirm=true 才能执行批量删除")
	}

	result, err := g.DB("master").Model("case_list").Ctx(ctx).Delete()
	if err != nil {
		return nil, err
	}

	deleted, _ := result.RowsAffected()
	return &v1.AdminCaseDeleteAllRes{Deleted: int(deleted)}, nil
}

// AdminCaseStatistics 获取案件统计数据
func (c *ControllerV1) AdminCaseStatistics(ctx context.Context, req *v1.AdminCaseStatisticsReq) (res *v1.AdminCaseStatisticsRes, err error) {

	db := g.DB("master")

	// 获取总数
	total, _ := db.Model("case_list").Ctx(ctx).Count()

	// 按案件类型统计
	byTypeRecords, _ := db.Model("case_list").Ctx(ctx).
		Fields("case_type as name, COUNT(*) as count").
		Where("case_type != ''").
		Group("case_type").
		OrderDesc("count").
		Limit(10).
		All()
	byType := make([]v1.CaseTypeStat, 0)
	for _, r := range byTypeRecords {
		byType = append(byType, v1.CaseTypeStat{
			Name:  r["name"].String(),
			Count: r["count"].Int(),
		})
	}

	// 按区域统计
	byRegionRecords, _ := db.Model("case_list").Ctx(ctx).
		Fields("region as name, COUNT(*) as count").
		Where("region != ''").
		Group("region").
		OrderDesc("count").
		Limit(10).
		All()
	byRegion := make([]v1.CaseTypeStat, 0)
	for _, r := range byRegionRecords {
		byRegion = append(byRegion, v1.CaseTypeStat{
			Name:  r["name"].String(),
			Count: r["count"].Int(),
		})
	}

	// 按来源统计
	bySourceRecords, _ := db.Model("case_list").Ctx(ctx).
		Fields("case_source as name, COUNT(*) as count").
		Where("case_source != ''").
		Group("case_source").
		OrderDesc("count").
		Limit(10).
		All()
	bySource := make([]v1.CaseTypeStat, 0)
	for _, r := range bySourceRecords {
		bySource = append(bySource, v1.CaseTypeStat{
			Name:  r["name"].String(),
			Count: r["count"].Int(),
		})
	}

	// 按待办环节统计
	byPendingRecords, _ := db.Model("case_list").Ctx(ctx).
		Fields("pending_step as name, COUNT(*) as count").
		Where("pending_step != ''").
		Group("pending_step").
		OrderDesc("count").
		Limit(10).
		All()
	byPending := make([]v1.CaseTypeStat, 0)
	for _, r := range byPendingRecords {
		byPending = append(byPending, v1.CaseTypeStat{
			Name:  r["name"].String(),
			Count: r["count"].Int(),
		})
	}

	return &v1.AdminCaseStatisticsRes{
		Statistics: v1.CaseStatistics{
			Total:     total,
			ByType:    byType,
			ByRegion:  byRegion,
			BySource:  bySource,
			ByPending: byPending,
		},
	}, nil
}

func (c *ControllerV1) AdminTopicKnowledgeBindings(ctx context.Context, req *v1.AdminTopicKnowledgeBindingsReq) (res *v1.AdminTopicKnowledgeBindingsRes, err error) {
	model := g.DB("master").Model("topic_knowledge_binding").Ctx(ctx)
	if req.Topic != "" {
		model = model.Where("topic = ?", req.Topic)
	}
	records, err := model.OrderAsc("topic").OrderAsc("knowledge_code").All()
	if err != nil {
		return nil, err
	}
	kbMap := make(map[string]string)
	kbRecords, _ := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Fields("code,name").All()
	for _, r := range kbRecords {
		kbMap[r["code"].String()] = r["name"].String()
	}
	list := make([]v1.TopicKnowledgeBindingItem, 0, len(records))
	for _, r := range records {
		code := r["knowledge_code"].String()
		list = append(list, v1.TopicKnowledgeBindingItem{
			Id:            r["id"].Int(),
			Topic:         r["topic"].String(),
			KnowledgeCode: code,
			KnowledgeName: kbMap[code],
			Enabled:       r["enabled"].Int() == 1,
			CreateTime:    r["create_time"].Int(),
			UpdateTime:    r["update_time"].Int(),
		})
	}
	return &v1.AdminTopicKnowledgeBindingsRes{List: list}, nil
}

func (c *ControllerV1) AdminBindTopicKnowledge(ctx context.Context, req *v1.AdminBindTopicKnowledgeReq) (res *v1.AdminBindTopicKnowledgeRes, err error) {
	now := int(gtime.Timestamp())
	count, err := g.DB("master").Model("topic_knowledge_binding").Ctx(ctx).
		Where("topic = ? AND knowledge_code = ?", req.Topic, req.KnowledgeCode).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		_, err = g.DB("master").Model("topic_knowledge_binding").Ctx(ctx).
			Where("topic = ? AND knowledge_code = ?", req.Topic, req.KnowledgeCode).
			Data(g.Map{"enabled": 1, "update_time": now}).Update()
		if err != nil {
			return nil, err
		}
	} else {
		_, err = g.DB("master").Model("topic_knowledge_binding").Ctx(ctx).Data(g.Map{
			"topic":          req.Topic,
			"knowledge_code": req.KnowledgeCode,
			"enabled":        1,
			"create_time":    now,
			"update_time":    now,
		}).Insert()
		if err != nil {
			return nil, err
		}
	}
	bind, _ := g.DB("master").Model("topic_knowledge_binding").Ctx(ctx).
		Where("topic = ? AND knowledge_code = ?", req.Topic, req.KnowledgeCode).One()
	kbMap := make(map[string]string)
	kbRecords, _ := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).Fields("code,name").All()
	for _, r := range kbRecords {
		kbMap[r["code"].String()] = r["name"].String()
	}
	item := v1.TopicKnowledgeBindingItem{
		Id:            bind["id"].Int(),
		Topic:         req.Topic,
		KnowledgeCode: req.KnowledgeCode,
		KnowledgeName: kbMap[req.KnowledgeCode],
		Enabled:       true,
		CreateTime:    now,
		UpdateTime:    now,
	}
	return &v1.AdminBindTopicKnowledgeRes{Item: item}, nil
}

func (c *ControllerV1) AdminUnbindTopicKnowledge(ctx context.Context, req *v1.AdminUnbindTopicKnowledgeReq) (res *v1.AdminUnbindTopicKnowledgeRes, err error) {
	_, err = g.DB("master").Model("topic_knowledge_binding").Ctx(ctx).Where("id = ?", req.Id).Delete()
	if err != nil {
		return nil, err
	}
	return &v1.AdminUnbindTopicKnowledgeRes{}, nil
}

func (c *ControllerV1) AdminToggleTopicKnowledge(ctx context.Context, req *v1.AdminToggleTopicKnowledgeReq) (res *v1.AdminToggleTopicKnowledgeRes, err error) {
	now := int(gtime.Timestamp())
	enabled := 0
	if req.Enabled {
		enabled = 1
	}
	_, err = g.DB("master").Model("topic_knowledge_binding").Ctx(ctx).
		Where("id = ?", req.Id).Data(g.Map{"enabled": enabled, "update_time": now}).Update()
	if err != nil {
		return nil, err
	}
	return &v1.AdminToggleTopicKnowledgeRes{Enabled: req.Enabled}, nil
}

func (c *ControllerV1) AdminUpdateDocumentVersion(ctx context.Context, req *v1.AdminUpdateDocumentVersionReq) (res *v1.AdminUpdateDocumentVersionRes, err error) {
	now := int(gtime.Timestamp())
	data := g.Map{"update_time": now}
	if req.EffectiveDate != "" {
		data["effective_date"] = req.EffectiveDate
	}
	if req.RepealDate != "" {
		data["repeal_date"] = req.RepealDate
	}
	if req.RepealedBy != "" {
		data["repealed_by"] = req.RepealedBy
	}
	if req.TitleGroup != "" {
		data["title_group"] = req.TitleGroup
	}
	if req.Status != "" {
		data["status"] = req.Status
	}
	_, err = g.DB("master").Model("qa_document").Ctx(ctx).
		Where("id = ?", req.Id).Data(data).Update()
	if err != nil {
		return nil, err
	}
	return &v1.AdminUpdateDocumentVersionRes{DocumentId: req.Id}, nil
}

func (c *ControllerV1) AdminDocumentVersions(ctx context.Context, req *v1.AdminDocumentVersionsReq) (res *v1.AdminDocumentVersionsRes, err error) {
	model := g.DB("master").Model("qa_document").Ctx(ctx).
		Fields("id, title, status, effective_date, repeal_date, repealed_by, title_group").
		Where("title_group IS NOT NULL AND title_group <> ''").
		OrderAsc("title_group").OrderAsc("effective_date")
	if req.TitleGroup != "" {
		model = model.Where("title_group = ?", req.TitleGroup)
	}
	records, err := model.All()
	if err != nil {
		return nil, err
	}
	groupMap := make(map[string][]v1.DocumentVersionDetail)
	order := make([]string, 0)
	for _, r := range records {
		tg := r["title_group"].String()
		if _, ok := groupMap[tg]; !ok {
			order = append(order, tg)
		}
		groupMap[tg] = append(groupMap[tg], v1.DocumentVersionDetail{
			DocumentId:    r["id"].Int64(),
			Title:         r["title"].String(),
			Status:        r["status"].String(),
			EffectiveDate: r["effective_date"].String(),
			RepealDate:    r["repeal_date"].String(),
			RepealedBy:    r["repealed_by"].String(),
		})
	}
	groups := make([]v1.DocumentVersionGroup, 0, len(order))
	for _, tg := range order {
		groups = append(groups, v1.DocumentVersionGroup{
			TitleGroup: tg,
			Versions:   groupMap[tg],
		})
	}
	return &v1.AdminDocumentVersionsRes{Groups: groups}, nil
}

func disableOrphanedKnowledgeBases(ctx context.Context, result aidgp.SyncResult) {
	syncedCodes := make(map[string]bool)
	for _, log := range result.Logs {
		if log.LocalId != "" && (log.Status == "success" || log.Status == "skipped") {
			syncedCodes[log.LocalId] = true
		}
	}
	if len(syncedCodes) == 0 {
		return
	}
	records, err := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).
		Fields("code").Where("source_provider = ?", "aidgp").All()
	if err != nil {
		return
	}
	now := int(gtime.Timestamp())
	for _, r := range records {
		code := r["code"].String()
		if !syncedCodes[code] {
			if _, kbErr := g.DB("master").Model("qa_knowledge_base").Ctx(ctx).
				Where("code = ?", code).Data(g.Map{"enabled": 0, "update_time": now}).Update(); kbErr != nil {
				consts.Logger.Warningf(ctx, "禁用知识库失败 code=%s: %s", code, kbErr.Error())
			}
		}
	}
}

func (c *ControllerV1) AdminDocumentRelations(ctx context.Context, req *v1.AdminDocumentRelationsReq) (res *v1.AdminDocumentRelationsRes, err error) {
	model := g.DB("master").Model("qa_document_relation r").Ctx(ctx).
		Fields("r.id, r.from_doc_id, r.to_doc_id, r.rel_type, r.description, r.enabled, f.title AS from_title, t.title AS to_title").
		LeftJoin("qa_document f", "f.id = r.from_doc_id").
		LeftJoin("qa_document t", "t.id = r.to_doc_id").
		OrderDesc("r.id")
	if req.DocumentId > 0 {
		model = model.Where("r.from_doc_id = ? OR r.to_doc_id = ?", req.DocumentId, req.DocumentId)
	}
	if req.RelType != "" {
		model = model.Where("r.rel_type = ?", req.RelType)
	}
	records, err := model.All()
	if err != nil {
		return nil, err
	}
	list := make([]v1.DocumentRelationItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.DocumentRelationItem{
			Id:           r["id"].Int64(),
			FromDocId:    r["from_doc_id"].Int64(),
			FromDocTitle: r["from_title"].String(),
			ToDocId:      r["to_doc_id"].Int64(),
			ToDocTitle:   r["to_title"].String(),
			RelType:      r["rel_type"].String(),
			Description:  r["description"].String(),
			Enabled:      r["enabled"].Int() == 1,
		})
	}
	return &v1.AdminDocumentRelationsRes{List: list}, nil
}

func (c *ControllerV1) AdminCreateDocumentRelation(ctx context.Context, req *v1.AdminCreateDocumentRelationReq) (res *v1.AdminCreateDocumentRelationRes, err error) {
	now := int(gtime.Timestamp())
	result, err := g.DB("master").Model("qa_document_relation").Ctx(ctx).Data(g.Map{
		"from_doc_id": req.FromDocId,
		"to_doc_id":   req.ToDocId,
		"rel_type":    req.RelType,
		"description": req.Description,
		"enabled":     1,
		"create_time": now,
		"update_time": now,
	}).Insert()
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &v1.AdminCreateDocumentRelationRes{Id: id}, nil
}

func (c *ControllerV1) AdminDeleteDocumentRelation(ctx context.Context, req *v1.AdminDeleteDocumentRelationReq) (res *v1.AdminDeleteDocumentRelationRes, err error) {
	_, err = g.DB("master").Model("qa_document_relation").Ctx(ctx).Where("id = ?", req.Id).Delete()
	return nil, err
}

func (c *ControllerV1) AdminDocumentRecommendations(ctx context.Context, req *v1.AdminDocumentRecommendationsReq) (res *v1.AdminDocumentRecommendationsRes, err error) {
	topN := req.TopN
	if topN <= 0 {
		topN = 5
	}
	records, err := g.DB("master").Model("qa_document_relation r").Ctx(ctx).
		Fields("r.id, r.from_doc_id, r.to_doc_id, r.rel_type, r.description, r.enabled, f.title AS from_title, t.title AS to_title").
		LeftJoin("qa_document f", "f.id = r.from_doc_id").
		LeftJoin("qa_document t", "t.id = r.to_doc_id").
		Where("(r.from_doc_id = ? OR r.to_doc_id = ?) AND r.enabled = 1", req.Id, req.Id).
		OrderDesc("r.id").
		Limit(topN).
		All()
	if err != nil {
		return nil, err
	}
	list := make([]v1.DocumentRelationItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.DocumentRelationItem{
			Id:           r["id"].Int64(),
			FromDocId:    r["from_doc_id"].Int64(),
			FromDocTitle: r["from_title"].String(),
			ToDocId:      r["to_doc_id"].Int64(),
			ToDocTitle:   r["to_title"].String(),
			RelType:      r["rel_type"].String(),
			Description:  r["description"].String(),
			Enabled:      r["enabled"].Int() == 1,
		})
	}
	return &v1.AdminDocumentRecommendationsRes{List: list}, nil
}

func (c *ControllerV1) AdminAutoDiscoverRelations(ctx context.Context, req *v1.AdminAutoDiscoverRelationsReq) (res *v1.AdminAutoDiscoverRelationsRes, err error) {
	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}

	model := g.DB("master").Model("qa_document d").Ctx(ctx).
		Fields("d.id, d.knowledge_code, d.title, d.doc_type, d.effective_date, d.repeal_date, d.repealed_by, d.title_group, d.status, kb.doc_type AS kb_doc_type").
		LeftJoin("qa_knowledge_base kb", "kb.code = d.knowledge_code").
		Where("d.status = ?", "active")
	if req.KnowledgeCode != "" {
		model = model.Where("d.knowledge_code = ?", req.KnowledgeCode)
	}

	records, err := model.OrderAsc("d.id").All()
	if err != nil {
		return nil, err
	}

	existingRels, err := g.DB("master").Model("qa_document_relation").Ctx(ctx).
		Fields("from_doc_id, to_doc_id, rel_type").All()
	if err != nil {
		return nil, err
	}
	existSet := make(map[string]bool, len(existingRels))
	for _, r := range existingRels {
		key := fmt.Sprintf("%d-%d-%s", r["from_doc_id"].Int64(), r["to_doc_id"].Int64(), r["rel_type"].String())
		existSet[key] = true
	}

	candidates := make([]v1.AutoDiscoverItem, 0)

	titleGroups := make(map[string][]gdb.Record)
	for _, r := range records {
		tg := r["title_group"].String()
		if tg != "" {
			titleGroups[tg] = append(titleGroups[tg], r)
		}
	}

	for _, group := range titleGroups {
		if len(group) < 2 {
			continue
		}
		sorted := make([]gdb.Record, len(group))
		copy(sorted, group)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i]["effective_date"].String() > sorted[j]["effective_date"].String()
		})
		for i := 0; i < len(sorted)-1; i++ {
			from := sorted[i]
			to := sorted[i+1]
			key := fmt.Sprintf("%d-%d-supplement", from["id"].Int64(), to["id"].Int64())
			if !existSet[key] {
				candidates = append(candidates, v1.AutoDiscoverItem{
					FromDocId:    from["id"].Int64(),
					FromDocTitle: from["title"].String(),
					ToDocId:      to["id"].Int64(),
					ToDocTitle:   to["title"].String(),
					RelType:      "supplement",
					Reason:       fmt.Sprintf("同名文件不同版本（%s → %s）", from["effective_date"].String(), to["effective_date"].String()),
				})
				existSet[key] = true
			}
		}
	}

	for _, r := range records {
		revokedBy := r["repealed_by"].String()
		if revokedBy == "" {
			continue
		}
		var targetId int64
		var targetTitle string
		if _, pErr := fmt.Sscanf(revokedBy, "%d", &targetId); pErr == nil && targetId > 0 {
			targetTitle = revokedBy
		} else {
			for _, t := range records {
				if t["title"].String() == revokedBy {
					targetId = t["id"].Int64()
					targetTitle = revokedBy
					break
				}
			}
		}
		if targetId <= 0 {
			continue
		}
		fromId := targetId
		toId := r["id"].Int64()
		key := fmt.Sprintf("%d-%d-repeal", fromId, toId)
		if !existSet[key] {
			candidates = append(candidates, v1.AutoDiscoverItem{
				FromDocId:    fromId,
				FromDocTitle: targetTitle,
				ToDocId:      toId,
				ToDocTitle:   r["title"].String(),
				RelType:      "repeal",
				Reason:       fmt.Sprintf("废止关系：'%s' 废止了 '%s'", targetTitle, r["title"].String()),
			})
			existSet[key] = true
		}
	}

	type docTopicKey struct {
		knowledgeCode string
		docType       string
	}
	topicDocs := make(map[docTopicKey][]gdb.Record)
	for _, r := range records {
		key := docTopicKey{
			knowledgeCode: r["knowledge_code"].String(),
			docType:       r["doc_type"].String(),
		}
		topicDocs[key] = append(topicDocs[key], r)
	}
	for key, docs := range topicDocs {
		if len(docs) < 2 {
			continue
		}
		for i := 0; i < len(docs) && i < 20; i++ {
			for j := i + 1; j < len(docs) && j < 20; j++ {
				a, b := docs[i], docs[j]
				relKey := fmt.Sprintf("%d-%d-related", a["id"].Int64(), b["id"].Int64())
				relKeyRev := fmt.Sprintf("%d-%d-related", b["id"].Int64(), a["id"].Int64())
				if existSet[relKey] || existSet[relKeyRev] {
					continue
				}
				candidates = append(candidates, v1.AutoDiscoverItem{
					FromDocId:    a["id"].Int64(),
					FromDocTitle: a["title"].String(),
					ToDocId:      b["id"].Int64(),
					ToDocTitle:   b["title"].String(),
					RelType:      "related",
					Reason:       fmt.Sprintf("同知识库同类型（%s/%s）", key.knowledgeCode, key.docType),
				})
				existSet[relKey] = true
			}
		}
	}

	var created int
	if !dryRun {
		now := int(gtime.Timestamp())
		for _, c := range candidates {
			_, insertErr := g.DB("master").Model("qa_document_relation").Ctx(ctx).Data(g.Map{
				"from_doc_id": c.FromDocId,
				"to_doc_id":   c.ToDocId,
				"rel_type":    c.RelType,
				"description": c.Reason,
				"enabled":     1,
				"create_time": now,
				"update_time": now,
			}).Insert()
			if insertErr == nil {
				created++
			}
		}
	}

	return &v1.AdminAutoDiscoverRelationsRes{Discovered: candidates, Created: created}, nil
}

func (c *ControllerV1) AdminComplianceCheck(ctx context.Context, req *v1.AdminComplianceCheckReq) (res *v1.AdminComplianceCheckRes, err error) {
	model := g.DB("master").Model("qa_document d").Ctx(ctx).
		Fields("d.id, d.title, d.knowledge_code, d.status, d.effective_date, d.repeal_date, d.repealed_by, d.title_group, d.doc_type").
		Where("d.status <> ?", "deleted")
	if req.KnowledgeCode != "" {
		model = model.Where("d.knowledge_code = ?", req.KnowledgeCode)
	}
	records, err := model.All()
	if err != nil {
		return nil, err
	}

	issues := make([]v1.ComplianceIssue, 0)

	now := gtime.Now()
	nowStr := now.Format("Y-m-d")

	for _, r := range records {
		if r["effective_date"].String() == "" && r["doc_type"].String() == "policy" {
			issues = append(issues, v1.ComplianceIssue{
				DocumentId:    r["id"].Int64(),
				DocumentTitle: r["title"].String(),
				IssueType:     "missing_effective_date",
				Description:   "政策制度类文档缺少生效日期",
				Severity:      "medium",
			})
		}

		repealDate := r["repeal_date"].String()
		if repealDate != "" && repealDate < nowStr && r["status"].String() == "active" {
			issues = append(issues, v1.ComplianceIssue{
				DocumentId:    r["id"].Int64(),
				DocumentTitle: r["title"].String(),
				IssueType:     "overdue_active",
				Description:   fmt.Sprintf("文档已过废止日期(%s)但仍为活跃状态", repealDate),
				Severity:      "high",
			})
		}

		revokedBy := r["repealed_by"].String()
		if revokedBy != "" {
			var refExists bool
			for _, t := range records {
				if fmt.Sprintf("%d", t["id"].Int64()) == revokedBy || t["title"].String() == revokedBy {
					refExists = true
					break
				}
			}
			if !refExists {
				issues = append(issues, v1.ComplianceIssue{
					DocumentId:    r["id"].Int64(),
					DocumentTitle: r["title"].String(),
					IssueType:     "broken_repeal_ref",
					Description:   fmt.Sprintf("废止引用'%s'在知识库中找不到对应文档", revokedBy),
					Severity:      "high",
				})
			}
		}

		tg := r["title_group"].String()
		if tg != "" && r["effective_date"].String() == "" {
			issues = append(issues, v1.ComplianceIssue{
				DocumentId:    r["id"].Int64(),
				DocumentTitle: r["title"].String(),
				IssueType:     "version_no_date",
				Description:   fmt.Sprintf("多版本文档'%s'缺少生效日期，无法确定版本顺序", tg),
				Severity:      "medium",
			})
		}
	}

	relCount, err := g.DB("master").Model("qa_document_relation r").Ctx(ctx).
		LeftJoin("qa_document d", "d.id = r.from_doc_id").
		Where("d.status = ?", "active").Count()
	if err == nil && relCount == 0 && len(records) > 5 {
		issues = append(issues, v1.ComplianceIssue{
			DocumentId:    0,
			DocumentTitle: "",
			IssueType:     "no_relations",
			Description:   "知识库中已有多个文档但无任何关联关系，建议运行自动发现",
			Severity:      "low",
		})
	}

	return &v1.AdminComplianceCheckRes{Issues: issues}, nil
}
