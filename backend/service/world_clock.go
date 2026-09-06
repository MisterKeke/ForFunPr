package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
)

type WorldClock struct {
	ID                     int    `json:"id"`
	Label                  string `json:"label"`
	TimeZoneID             string `json:"time_zone_id"`
	SortOrder              int    `json:"sort_order"`
	WorkingDayStartMinutes int    `json:"working_day_start_minutes"`
	WorkingDayEndMinutes   int    `json:"working_day_end_minutes"`
	CurrentTime            string `json:"current_time"`
	UTCOffsetSeconds       int    `json:"utc_offset_seconds"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
}

type WorldClockWriteRequest struct {
	ID                     int    `json:"id"`
	Label                  string `json:"label"`
	TimeZoneID             string `json:"time_zone_id"`
	WorkingDayStartMinutes int    `json:"working_day_start_minutes"`
	WorkingDayEndMinutes   int    `json:"working_day_end_minutes"`
}

type WorldClockOrderRequest struct {
	IDs []int `json:"ids"`
}

type TimeZoneOption struct {
	ID     string `json:"id"`
	City   string `json:"city"`
	Region string `json:"region"`
}

type WorldTimeConversionRequest struct {
	At         string   `json:"at"`
	FromZoneID string   `json:"from_zone_id"`
	ToZoneIDs  []string `json:"to_zone_ids"`
}

type WorldTimeConversion struct {
	TimeZoneID       string   `json:"time_zone_id"`
	LocalTime        string   `json:"local_time"`
	UTCOffsetSeconds int      `json:"utc_offset_seconds"`
	Ambiguous        bool     `json:"ambiguous"`
	Alternatives     []string `json:"alternatives"`
}

var bundledTimeZones = []string{
	"Africa/Cairo", "Africa/Casablanca", "Africa/Johannesburg", "Africa/Lagos", "Africa/Nairobi",
	"America/Anchorage", "America/Argentina/Buenos_Aires", "America/Bogota", "America/Chicago",
	"America/Denver", "America/Halifax", "America/Lima", "America/Los_Angeles", "America/Mexico_City",
	"America/New_York", "America/Phoenix", "America/Santiago", "America/Sao_Paulo", "America/St_Johns",
	"America/Toronto", "America/Vancouver", "Asia/Almaty", "Asia/Baghdad", "Asia/Baku", "Asia/Bangkok",
	"Asia/Beirut", "Asia/Colombo", "Asia/Dhaka", "Asia/Dubai", "Asia/Hong_Kong", "Asia/Jakarta",
	"Asia/Jerusalem", "Asia/Karachi", "Asia/Kathmandu", "Asia/Kolkata", "Asia/Kuala_Lumpur",
	"Asia/Manila", "Asia/Riyadh", "Asia/Seoul", "Asia/Shanghai", "Asia/Singapore", "Asia/Taipei",
	"Asia/Tashkent", "Asia/Tbilisi", "Asia/Tehran", "Asia/Tokyo", "Asia/Yerevan", "Atlantic/Reykjavik",
	"Australia/Adelaide", "Australia/Brisbane", "Australia/Darwin", "Australia/Hobart", "Australia/Melbourne",
	"Australia/Perth", "Australia/Sydney", "Europe/Amsterdam", "Europe/Athens", "Europe/Belgrade",
	"Europe/Berlin", "Europe/Brussels", "Europe/Bucharest", "Europe/Budapest", "Europe/Copenhagen",
	"Europe/Dublin", "Europe/Helsinki", "Europe/Istanbul", "Europe/Kyiv", "Europe/Lisbon", "Europe/London",
	"Europe/Madrid", "Europe/Moscow", "Europe/Oslo", "Europe/Paris", "Europe/Prague", "Europe/Rome",
	"Europe/Sofia", "Europe/Stockholm", "Europe/Tallinn", "Europe/Vienna", "Europe/Vilnius", "Europe/Warsaw",
	"Europe/Zurich", "Pacific/Auckland", "Pacific/Fiji", "Pacific/Honolulu", "Pacific/Port_Moresby",
	"Pacific/Tahiti", "UTC",
}

func (a *Service) ListTimeZonesContext(context.Context) []TimeZoneOption {
	options := make([]TimeZoneOption, 0, len(bundledTimeZones))
	for _, id := range bundledTimeZones {
		parts := strings.Split(id, "/")
		city := strings.ReplaceAll(parts[len(parts)-1], "_", " ")
		region := "Universal"
		if len(parts) > 1 {
			region = parts[0]
		}
		options = append(options, TimeZoneOption{ID: id, City: city, Region: region})
	}
	return options
}

func (a *Service) ListWorldClocksContext(ctx context.Context) ([]WorldClock, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, label, time_zone_id, sort_order, working_day_start_minutes,
		       working_day_end_minutes, created_at, updated_at
		FROM world_clocks ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list world clocks: %w", err)
	}
	defer rows.Close()
	clocks := make([]WorldClock, 0)
	now := time.Now()
	for rows.Next() {
		var clock WorldClock
		if err := rows.Scan(&clock.ID, &clock.Label, &clock.TimeZoneID, &clock.SortOrder,
			&clock.WorkingDayStartMinutes, &clock.WorkingDayEndMinutes, &clock.CreatedAt, &clock.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan world clock: %w", err)
		}
		location, err := time.LoadLocation(clock.TimeZoneID)
		if err != nil {
			return nil, fmt.Errorf("load stored time zone %q: %w", clock.TimeZoneID, err)
		}
		local := now.In(location)
		_, clock.UTCOffsetSeconds = local.Zone()
		clock.CurrentTime = local.Format(time.RFC3339)
		clocks = append(clocks, clock)
	}
	return clocks, rows.Err()
}

func (a *Service) CreateWorldClockContext(ctx context.Context, request WorldClockWriteRequest) (WorldClock, error) {
	normalized, err := normalizeWorldClockRequest(request)
	if err != nil {
		return WorldClock{}, err
	}
	var count int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM world_clocks`).Scan(&count); err != nil {
		return WorldClock{}, fmt.Errorf("count world clocks: %w", err)
	}
	if count >= 100 {
		return WorldClock{}, &ValidationError{Field: "time_zone_id", Message: "a maximum of 100 world clocks can be saved"}
	}
	var order int
	if err := a.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM world_clocks`).Scan(&order); err != nil {
		return WorldClock{}, fmt.Errorf("choose world clock order: %w", err)
	}
	result, err := a.db.ExecContext(ctx, `
		INSERT INTO world_clocks (
			label, time_zone_id, sort_order, working_day_start_minutes, working_day_end_minutes
		) VALUES (?, ?, ?, ?, ?)
	`, normalized.Label, normalized.TimeZoneID, order, normalized.WorkingDayStartMinutes, normalized.WorkingDayEndMinutes)
	if err != nil {
		return WorldClock{}, fmt.Errorf("create world clock: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return WorldClock{}, fmt.Errorf("read world clock ID: %w", err)
	}
	return a.getWorldClockContext(ctx, int(id))
}

func (a *Service) UpdateWorldClockContext(ctx context.Context, request WorldClockWriteRequest) (WorldClock, error) {
	if request.ID <= 0 {
		return WorldClock{}, &ValidationError{Field: "id", Message: "world clock ID must be positive"}
	}
	normalized, err := normalizeWorldClockRequest(request)
	if err != nil {
		return WorldClock{}, err
	}
	result, err := a.db.ExecContext(ctx, `
		UPDATE world_clocks SET label = ?, time_zone_id = ?, working_day_start_minutes = ?,
		       working_day_end_minutes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, normalized.Label, normalized.TimeZoneID, normalized.WorkingDayStartMinutes, normalized.WorkingDayEndMinutes, request.ID)
	if err != nil {
		return WorldClock{}, fmt.Errorf("update world clock: %w", err)
	}
	if err := requireSingleMutation(result, "update world clock", "world clock", false); err != nil {
		return WorldClock{}, err
	}
	return a.getWorldClockContext(ctx, request.ID)
}

func (a *Service) DeleteWorldClockContext(ctx context.Context, id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "world clock ID must be positive"}
	}
	result, err := a.db.ExecContext(ctx, `DELETE FROM world_clocks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete world clock: %w", err)
	}
	return requireSingleMutation(result, "delete world clock", "world clock", false)
}

func (a *Service) ReorderWorldClocksContext(ctx context.Context, request WorldClockOrderRequest) error {
	clocks, err := a.ListWorldClocksContext(ctx)
	if err != nil {
		return err
	}
	if len(clocks) != len(request.IDs) {
		return &ValidationError{Field: "ids", Message: "order must contain every saved world clock"}
	}
	existing := make(map[int]bool, len(clocks))
	for _, clock := range clocks {
		existing[clock.ID] = true
	}
	seen := make(map[int]bool, len(request.IDs))
	for _, id := range request.IDs {
		if !existing[id] || seen[id] {
			return &ValidationError{Field: "ids", Message: "order contains an unknown or duplicate world clock"}
		}
		seen[id] = true
	}
	transaction, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin world clock reorder: %w", err)
	}
	defer transaction.Rollback()
	for order, id := range request.IDs {
		if _, err := transaction.ExecContext(ctx, `UPDATE world_clocks SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, order, id); err != nil {
			return fmt.Errorf("reorder world clock: %w", err)
		}
	}
	return transaction.Commit()
}

func (a *Service) ConvertWorldTimeContext(ctx context.Context, request WorldTimeConversionRequest) ([]WorldTimeConversion, error) {
	from, err := time.LoadLocation(strings.TrimSpace(request.FromZoneID))
	if err != nil {
		return nil, &ValidationError{Field: "from_zone_id", Message: "source time zone is not valid"}
	}
	wall := strings.TrimSpace(request.At)
	instant, err := time.Parse(time.RFC3339, wall)
	if err != nil {
		instant, err = time.ParseInLocation("2006-01-02T15:04", wall, from)
		if err != nil {
			return nil, &ValidationError{Field: "at", Message: "time must use YYYY-MM-DDTHH:MM or RFC3339"}
		}
		if instant.In(from).Format("2006-01-02T15:04") != wall {
			return nil, &ValidationError{Field: "at", Message: "that local time does not exist because of a daylight-saving transition"}
		}
		for _, shift := range []time.Duration{
			-2 * time.Hour, -90 * time.Minute, -time.Hour, -30 * time.Minute,
			30 * time.Minute, time.Hour, 90 * time.Minute, 2 * time.Hour,
		} {
			alternative := instant.Add(shift)
			if alternative.In(from).Format("2006-01-02T15:04") == wall {
				return nil, &ValidationError{Field: "at", Message: "that local time is ambiguous because of a daylight-saving transition; enter an RFC3339 time with an explicit offset"}
			}
		}
	}
	zoneIDs := request.ToZoneIDs
	if len(zoneIDs) == 0 {
		clocks, listErr := a.ListWorldClocksContext(ctx)
		if listErr != nil {
			return nil, listErr
		}
		for _, clock := range clocks {
			zoneIDs = append(zoneIDs, clock.TimeZoneID)
		}
	}
	if len(zoneIDs) == 0 || len(zoneIDs) > 100 {
		return nil, &ValidationError{Field: "to_zone_ids", Message: "choose between 1 and 100 target time zones"}
	}
	results := make([]WorldTimeConversion, 0, len(zoneIDs))
	for _, zoneID := range zoneIDs {
		location, loadErr := time.LoadLocation(strings.TrimSpace(zoneID))
		if loadErr != nil {
			return nil, &ValidationError{Field: "to_zone_ids", Message: fmt.Sprintf("time zone %q is not valid", zoneID)}
		}
		local := instant.In(location)
		_, offset := local.Zone()
		results = append(results, WorldTimeConversion{TimeZoneID: zoneID, LocalTime: local.Format(time.RFC3339), UTCOffsetSeconds: offset})
	}
	return results, nil
}

func (a *Service) getWorldClockContext(ctx context.Context, id int) (WorldClock, error) {
	var clock WorldClock
	err := a.db.QueryRowContext(ctx, `
		SELECT id, label, time_zone_id, sort_order, working_day_start_minutes,
		       working_day_end_minutes, created_at, updated_at
		FROM world_clocks WHERE id = ?
	`, id).Scan(&clock.ID, &clock.Label, &clock.TimeZoneID, &clock.SortOrder,
		&clock.WorkingDayStartMinutes, &clock.WorkingDayEndMinutes, &clock.CreatedAt, &clock.UpdatedAt)
	if err == sql.ErrNoRows {
		return WorldClock{}, &NotFoundError{Resource: "world clock", Key: fmt.Sprint(id)}
	}
	if err != nil {
		return WorldClock{}, fmt.Errorf("read world clock: %w", err)
	}
	location, err := time.LoadLocation(clock.TimeZoneID)
	if err != nil {
		return WorldClock{}, fmt.Errorf("load world clock time zone: %w", err)
	}
	local := time.Now().In(location)
	_, clock.UTCOffsetSeconds = local.Zone()
	clock.CurrentTime = local.Format(time.RFC3339)
	return clock, nil
}

func normalizeWorldClockRequest(request WorldClockWriteRequest) (WorldClockWriteRequest, error) {
	request.Label = strings.TrimSpace(request.Label)
	request.TimeZoneID = strings.TrimSpace(request.TimeZoneID)
	if len([]rune(request.Label)) < 1 || len([]rune(request.Label)) > 80 {
		return WorldClockWriteRequest{}, &ValidationError{Field: "label", Message: "clock label must contain between 1 and 80 characters"}
	}
	if _, err := time.LoadLocation(request.TimeZoneID); err != nil {
		return WorldClockWriteRequest{}, &ValidationError{Field: "time_zone_id", Message: "choose a valid IANA time zone"}
	}
	if request.WorkingDayStartMinutes < 0 || request.WorkingDayStartMinutes > 1439 ||
		request.WorkingDayEndMinutes < 1 || request.WorkingDayEndMinutes > 1440 ||
		request.WorkingDayStartMinutes >= request.WorkingDayEndMinutes {
		return WorldClockWriteRequest{}, &ValidationError{Field: "working_hours", Message: "working hours must form a valid range within one day"}
	}
	return request, nil
}

func init() { sort.Strings(bundledTimeZones) }
