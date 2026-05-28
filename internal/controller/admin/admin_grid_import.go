package admin

import (
	"context"
	"fmt"
	"strings"

	v1 "ai-chat-sql/api/admin/v1"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/xuri/excelize/v2"
)

// AdminGridImportTemplate 下载网格数据导入模板（含9列必填表头 + 2行示例数据）
func (c *ControllerV1) AdminGridImportTemplate(ctx context.Context, req *v1.AdminGridImportTemplateReq) (res *v1.AdminGridImportTemplateRes, err error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	headers := []string{"责任单位", "案件编号", "案件来源", "上报时间", "待办环节", "案件类别", "所属区域", "案件位置", "问题描述"}
	today := gtime.Now().Format("Y-m-d")
	examples := [][]string{
		{"XXè¡éå", "CASE-2026-001", "12345ç­çº¿", today + " 09:30:00", "å¾å¤ç", "å¸å®¹ç¯å«", "XXç¤¾åº", "XXè·¯ä¸XXè·¯äº¤åå£", "è·¯é¢åå¾å ç§¯"},
		{"XXè¡éå", "CASE-2026-002", "ç½æ ¼å·¡æ¥", today + " 14:00:00", "å¤ç½®ä¸­", "å¸æ¿è®¾æ½", "YYç¤¾åº", "YYè·¯28å·", "è·¯ç¯æå"},
	}
	styleID, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	_ = f.SetCellStyle(sheet, "A1", "I1", styleID)
	for rowIdx, row := range examples {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheet, cell, val)
		}
	}
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, 18)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, gerror.New("failed to generate template file")
	}
	r := ghttp.RequestFromCtx(ctx)
	r.Response.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	r.Response.Header().Set("Content-Disposition", "attachment; filename=grid_import_template.xlsx")
	r.Response.Write(buf.Bytes())
	return
}

// AdminGridImportRollback 回滚一次导入：删除该批次的 case_list 记录，状态改为 rolled_back
func (c *ControllerV1) AdminGridImportRollback(ctx context.Context, req *v1.AdminGridImportRollbackReq) (res *v1.AdminGridImportRollbackRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("导入记录ID不能为空")
	}
	record, err := g.DB("master").Model("admin_grid_import").Ctx(ctx).Where("id = ?", req.Id).One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, gerror.New("导入记录不存在")
	}
	status := record["status"].String()
	if status == "rolled_back" {
		return nil, gerror.New("该批次已回滚，不可重复操作")
	}
	if status == "running" {
		return nil, gerror.New("该批次正在导入中，请等待完成后再回滚")
	}

	// 查出该批次导入的所有 case_number
	caseNumbers, err := g.DB("master").Model("admin_grid_import_error").Ctx(ctx).
		Fields("raw_data").Where("import_id = ? AND reason = ?", req.Id, "rollback_case_numbers").All()
	if err != nil {
		return nil, err
	}

	var deletedCount int64
	if len(caseNumbers) > 0 {
		nums := make([]string, 0, len(caseNumbers))
		for _, r := range caseNumbers {
			s := strings.TrimSpace(r["raw_data"].String())
			if s != "" {
				nums = append(nums, s)
			}
		}
		if len(nums) > 0 {
			result, delErr := g.DB("master").Model("case_list").Ctx(ctx).Where("case_number IN(?)", nums).Delete()
			if delErr != nil {
				return nil, delErr
			}
			deletedCount, _ = result.RowsAffected()
		}
	} else {
		// 旧批次没有 case_numbers 记录：按 month 匹配近似删除
		month := record["month"].String()
		if month != "" {
			result, delErr := g.DB("master").Model("case_list").Ctx(ctx).
				Where("DATE_FORMAT(report_time, '%Y-%m') = ?", month).Delete()
			if delErr != nil {
				return nil, delErr
			}
			deletedCount, _ = result.RowsAffected()
		}
	}

	now := int(gtime.Timestamp())
	_, err = g.DB("master").Model("admin_grid_import").Ctx(ctx).Where("id = ?", req.Id).Data(g.Map{
		"status":        "rolled_back",
		"complete_time": now,
	}).Update()
	if err != nil {
		return nil, err
	}

	op := defaultString(req.Operator, "admin")
	_, _ = g.DB("master").Model("admin_grid_import_audit").Ctx(ctx).Data(g.Map{
		"import_id":   req.Id,
		"action":      "rollback",
		"operator":    op,
		"detail":      fmt.Sprintf("回滚删除 %d 条案件记录", deletedCount),
		"create_time": now,
	}).Insert()

	_ = insertAdminLog(ctx, "admin", op, "数据导入回滚", fmt.Sprintf("回滚导入ID %d，删除 %d 条", req.Id, deletedCount), "success")

	item, err := getGridImport(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportRollbackRes{Item: item}, nil
}

// AdminGridImportAudit 查看某次导入的审计轨迹
func (c *ControllerV1) AdminGridImportAudit(ctx context.Context, req *v1.AdminGridImportAuditReq) (res *v1.AdminGridImportAuditRes, err error) {
	if req.Id <= 0 {
		return nil, gerror.New("导入记录ID不能为空")
	}
	records, err := g.DB("master").Model("admin_grid_import_audit").Ctx(ctx).
		Where("import_id = ?", req.Id).OrderAsc("create_time").All()
	if err != nil {
		return nil, err
	}
	return &v1.AdminGridImportAuditRes{List: scanGridImportAudits(records)}, nil
}

func scanGridImportAudits(records gdb.Result) []v1.GridImportAuditItem {
	list := make([]v1.GridImportAuditItem, 0, len(records))
	for _, r := range records {
		list = append(list, v1.GridImportAuditItem{
			Id:         r["id"].Int(),
			ImportId:   r["import_id"].Int(),
			Action:     r["action"].String(),
			Operator:   r["operator"].String(),
			Detail:     r["detail"].String(),
			CreateTime: r["create_time"].Int(),
		})
	}
	return list
}
