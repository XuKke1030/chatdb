package system

import (
	"ai-chat-sql/internal/controller/admin"
	"ai-chat-sql/internal/controller/ai_chat"
	"ai-chat-sql/internal/controller/qa"
	"ai-chat-sql/internal/logic/alert"
	"ai-chat-sql/internal/service"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

// InitDB initializes all database tables and seed data during service startup.
func (s *sSystemInit) InitDB(ctx context.Context) error {
	db := g.DB("master")

	// Admin must be created before QA indexes, because QA retrieval depends on
	// admin_user_knowledge_permission.
	if err := admin.CreateAdminTables(ctx, db); err != nil {
		return fmt.Errorf("admin table init failed: %w", err)
	}
	admin.MigrateAdminTables(ctx, db)
	_, _ = db.Exec(ctx, "ALTER TABLE admin_grid_import_error MODIFY COLUMN raw_data LONGTEXT")

	if err := qa.CreateQaTables(ctx, db); err != nil {
		return fmt.Errorf("qa table init failed: %w", err)
	}
	if err := ai_chat.CreateAskNumberSessionTables(ctx, db); err != nil {
		return fmt.Errorf("ask-number session table init failed: %w", err)
	}
	if err := alert.CreateAlertTables(ctx, db); err != nil {
		return fmt.Errorf("alert table init failed: %w", err)
	}
	if err := service.Traffic().InitTables(ctx); err != nil {
		return fmt.Errorf("traffic table init failed: %w", err)
	}

	if err := qa.SeedQaTables(ctx, db); err != nil {
		return fmt.Errorf("qa seed init failed: %w", err)
	}
	if err := admin.SeedAdminTables(ctx, db); err != nil {
		return fmt.Errorf("admin seed init failed: %w", err)
	}
	if err := alert.SeedAlertTables(ctx, db); err != nil {
		return fmt.Errorf("alert seed init failed: %w", err)
	}

	return nil
}
