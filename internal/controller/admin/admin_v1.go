package admin

import (
	"ai-chat-sql/internal/consts"
	"ai-chat-sql/internal/logic/aidgp"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
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
		return nil, gerror.New("绠＄悊鍛樿处鍙峰拰瀵嗙爜涓嶈兘涓虹┖")
	}
	record, err := g.DB("master").Model("admin_account").Ctx(ctx).Where("username = ? AND enabled = ?", req.Username, 1).One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, gerror.New("绠＄悊鍛樿处鍙锋垨瀵嗙爜閿欒")
	}
	verify := record["verify"].Int()
	password, err := genAdminPassword(req.Password, verify)
	if err != nil {
		return nil, err
	}
	if record["password"].String() != password {
		return nil, gerror.New("绠＄悊鍛樿处鍙锋垨瀵嗙爜閿欒")
	}
	_ = insertAdminLog(ctx, "admin", req.Username, "管理员登录", "管理员登录后台管理平台", "success")
	return &v1.AdminLoginRes{
		Token:    "admin-dev-token",
		Username: req.Username,
	}, nil
}

func (c *ControllerV1) AdminProfile(ctx context.Context, req *v1.AdminProfileReq) (res *v1.AdminProfileRes, err error) {
	return &v1.AdminProfileRes{Username: "admin", Role: "administrator"}, nil
}

func (c *ControllerV1) AdminUsers(ctx context.Context, req *v1.AdminUsersReq) (res *v1.AdminUsersRes, err error) {
	users, err := listAdminUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUsersRes{List: users}, nil
}

func (c *ControllerV1) AdminKnowledgeBases(ctx context.Context, req *v1.AdminKnowledgeBasesReq) (res *v1.AdminKnowledgeBasesRes, err error) {
	return &v1.AdminKnowledgeBasesRes{List: knowledgeBaseOptions(ctx)}, nil
}

func (c *ControllerV1) AdminUpdateUserPermissions(ctx context.Context, req *v1.AdminUpdateUserPermissionsReq) (res *v1.AdminUpdateUserPermissionsRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("鐢ㄦ埛ID涓嶈兘涓虹┖")
	}
	ruleLevel := req.RuleLevel
	if ruleLevel == 0 && len(req.Permissions) > 0 {
		ruleLevel = ruleLevelFromPermissions(req.Permissions)
	}
	now := int(gtime.Timestamp())
	_, _ = g.DB("master").Model("user").Ctx(ctx).Where("user_id = ?", req.Id).Data(g.Map{
		"rule_level":  ruleLevel,
		"update_time": now,
	}).Update()
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
		return nil, gerror.New("鐢ㄦ埛ID涓嶈兘涓虹┖")
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
	records, err := query.OrderAsc("sort").OrderDesc("update_time").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminExampleQuestionsRes{List: scanExampleQuestions(records)}, nil
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
		return nil, gerror.New("闂ID涓嶈兘涓虹┖")
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
		return nil, gerror.New("闂ID涓嶈兘涓虹┖")
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
	records, err := query.OrderDesc("last_seen_at").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminQuestionCandidatesRes{List: scanQuestionCandidates(records)}, nil
}

func (c *ControllerV1) AdminApproveQuestionCandidate(ctx context.Context, req *v1.AdminApproveQuestionCandidateReq) (res *v1.AdminApproveQuestionCandidateRes, err error) {
	return c.updateCandidateStatus(ctx, req.Id, "approved")
}

func (c *ControllerV1) AdminRejectQuestionCandidate(ctx context.Context, req *v1.AdminRejectQuestionCandidateReq) (res *v1.AdminRejectQuestionCandidateRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("鍊欓€夐棶棰業D涓嶈兘涓虹┖")
	}
	item, err := updateQuestionCandidateStatus(ctx, req.Id, "rejected")
	if err != nil {
		return nil, err
	}
	return &v1.AdminRejectQuestionCandidateRes{Item: item}, nil
}

func (c *ControllerV1) AdminDataSources(ctx context.Context, req *v1.AdminDataSourcesReq) (res *v1.AdminDataSourcesRes, err error) {
	records, err := g.DB("master").Model("admin_data_source").Ctx(ctx).OrderAsc("id").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminDataSourcesRes{List: scanDataSources(records)}, nil
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
	records, err := g.DB("master").Model("admin_grid_import").Ctx(ctx).OrderDesc("create_time").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportsRes{List: scanGridImports(records)}, nil
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

func (c *ControllerV1) AdminLogs(ctx context.Context, req *v1.AdminLogsReq) (res *v1.AdminLogsRes, err error) {
	query := g.DB("master").Model("admin_operation_log").Ctx(ctx)
	if req.LogType == "system" || req.LogType == "admin" {
		query = query.Where("log_type = ?", req.LogType)
	}
	records, err := query.OrderDesc("create_time").OrderDesc("id").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminLogsRes{List: scanAdminLogs(records)}, nil
}

func (c *ControllerV1) updateCandidateStatus(ctx context.Context, id int, status string) (*v1.AdminApproveQuestionCandidateRes, error) {
	if id <= 0 {
		return nil, gerror.New("鍊欓€夐棶棰業D涓嶈兘涓虹┖")
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
			{"topic": "grid", "question": "鏈湀楂樻柊鍖烘浠剁粨妗堢巼鏄灏戯紵", "description": "鏌ヨ缃戞牸妗堜欢鍔炵悊鎴愭晥", "enabled": 1, "sort": 10, "create_time": now, "update_time": now},
			{"topic": "population", "question": "杩囧幓涓€鍛ㄤ汉娴佽繘鍑鸿秼鍔垮浣曪紵", "description": "灞曠ず姣忔棩杩涘嚭浜烘暟瀵规瘮", "enabled": 1, "sort": 20, "create_time": now, "update_time": now},
			{"topic": "traffic", "question": "浠婃棩娓境杞﹁締鍗犳瘮鏄灏戯紵", "description": "缁熻閲嶇偣鍗″彛璺ㄥ杞﹁締鎯呭喌", "enabled": 1, "sort": 30, "create_time": now, "update_time": now},
		}
		for _, seed := range seeds {
			if _, err := db.Model("admin_example_question").Ctx(ctx).Data(seed).Insert(); err != nil {
				return err
			}
		}
	}
	if count, err := db.Model("admin_data_source").Ctx(ctx).Count(); err == nil && count == 0 {
		seeds := []g.Map{
			{"source_type": "population", "name": "浜烘祦鏁版嵁鎺ュ叆", "enabled": 0, "status": "closed", "latest_sync": 0, "create_time": now, "update_time": now},
			{"source_type": "traffic", "name": "杞︽祦鏁版嵁鎺ュ叆", "enabled": 0, "status": "closed", "latest_sync": 0, "create_time": now, "update_time": now},
			{"source_type": "grid", "name": "缃戞牸鏈堝害瀵煎叆", "enabled": 1, "status": "ready", "latest_sync": 0, "create_time": now, "update_time": now},
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
	return nil
}

func listAdminUsers(ctx context.Context) ([]v1.AdminUserItem, error) {
	records, err := g.DB("master").Model("user").Ctx(ctx).Fields("user_id, username, rule_level, last_login_tme, update_time").OrderAsc("user_id").All()
	if err != nil {
		return []v1.AdminUserItem{}, nil
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
	return list, nil
}

func getAdminUser(ctx context.Context, id int) (v1.AdminUserItem, error) {
	users, err := listAdminUsers(ctx)
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
		allowed := enabled && (code == "policy" || (ruleLevel&7 == 7 && code == "manual"))
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
	perms, _ := g.DB("master").Model("admin_user_knowledge_permission").Ctx(ctx).Where("user_id = ?", userId).All()
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
		return v1.DataSourceItem{}, gerror.New("鏁版嵁婧愪笉瀛樺湪")
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
		return v1.QuestionCandidateItem{}, gerror.New("鍊欓€夐棶棰樹笉瀛樺湪")
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
	return err
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
	} else if result.FailureCount > 0 {
		status = "partial_failed"
	} else if result.SuccessCount == 0 && result.SkippedCount > 0 {
		status = "skipped"
	}
	if err = insertAdminSyncLogs(ctx, taskId, provider, syncType, result.Logs); err != nil {
		return nil, err
	}
	finishedAt := int(gtime.Timestamp())
	if err = finishAdminSyncTask(ctx, taskId, status, message, result.SuccessCount, result.FailureCount, result.SkippedCount, finishedAt); err != nil {
		return nil, err
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
	if consts.Config == nil || consts.Config.QaConfig == nil || consts.Config.QaConfig.Sync == nil {
		return aidgp.ProviderMock
	}
	provider := strings.TrimSpace(consts.Config.QaConfig.Sync.Provider)
	if provider == "" {
		return aidgp.ProviderMock
	}
	return provider
}

func adminAidgpClient(provider string) aidgp.Client {
	cfg := aidgp.Config{Provider: provider}
	if consts.Config != nil && consts.Config.QaConfig != nil && consts.Config.QaConfig.Sync != nil && consts.Config.QaConfig.Sync.Aidgp != nil {
		aidgpCfg := consts.Config.QaConfig.Sync.Aidgp
		cfg.BaseUrl = aidgpCfg.BaseUrl
		cfg.AppKey = aidgpCfg.AppKey
		cfg.AppSecret = aidgpCfg.AppSecret
	}
	return aidgp.NewClient(cfg)
}

func callAdminAidgpSync(ctx context.Context, client aidgp.Client, syncType string, knowledgeCode string) (aidgp.SyncResult, error) {
	scope := aidgp.SyncScope{KnowledgeCode: knowledgeCode}
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
		db = db.WhereLike("case_number", "%"+req.CaseNumber+"%")
	}
	if req.CaseType != "" {
		db = db.WhereLike("case_type", "%"+req.CaseType+"%")
	}
	if req.Region != "" {
		db = db.WhereLike("region", "%"+req.Region+"%")
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
