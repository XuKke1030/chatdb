package traffic

import (
	"ai-chat-sql/internal/consts"
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

type holidayAPIResponse struct {
	Code int                        `json:"code"`
	Data map[string]holidayAPIDatum `json:"holiday"`
}

type holidayAPIDatum struct {
	Name    string `json:"name"`
	Holiday bool   `json:"holiday"`
	Date    string `json:"date"`
}

func (s *sTraffic) FetchHolidaysFromAPI(ctx context.Context, year int) error {
	url := fmt.Sprintf("https://timor.tech/api/holiday/year/%d", year)
	resp, err := g.Client().Get(ctx, url)
	if err != nil {
		g.Log().Infof(ctx, "FetchHolidaysFromAPI: HTTP error for year %d: %v", year, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result holidayAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		g.Log().Infof(ctx, "FetchHolidaysFromAPI: JSON parse error for year %d: %v", year, err)
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("API code %d", result.Code)
	}

	db := g.DB("master")
	for _, datum := range result.Data {
		holidayType := "workday"
		if datum.Holiday {
			holidayType = "holiday"
		}
		if _, err := db.Exec(ctx, `
INSERT INTO traffic_holiday (holiday_date, holiday_name, holiday_type)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE holiday_name = VALUES(holiday_name), holiday_type = VALUES(holiday_type)`,
			datum.Date, datum.Name, holidayType); err != nil {
			consts.Logger.Warningf(ctx, "holiday API sync failed for %s: %v", datum.Date, err)
		}
	}

	g.Log().Infof(ctx, "FetchHolidaysFromAPI: synced %d dates for year %d", len(result.Data), year)
	return nil
}

func autoSyncHolidays(ctx context.Context) {
	s := NewTraffic()
	currentYear := gtime.Now().Year()
	for _, y := range []int{currentYear, currentYear + 1} {
		if err := s.FetchHolidaysFromAPI(ctx, y); err != nil {
			g.Log().Infof(ctx, "autoSyncHolidays: API fetch failed for year %d: %v", y, err)
		}
	}
	_ = s.SyncHolidaysFromCode(ctx)
}
