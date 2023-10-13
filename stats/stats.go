package stats

import (
	"context"
	"encoding/base64"
	"fmt"
	"golang.org/x/oauth2/google"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"log"
	"time"
)

// StatsUpdater is a struct that holds the Google Sheets service, the spreadsheet ID, and a map of stat names to row numbers.
type StatsUpdater struct {
	srv           *sheets.Service
	spreadsheetID string
	statRowMap    map[string]int
}

// Rate limiter for Google Sheets API calls
var sheetsLimiter = rate.NewLimiter(rate.Every(time.Minute/29), 29)

// writeToSheetWithRateLimit waits for a token from the rate limiter, then writes values to a Google Sheet.
// It returns the response from the write operation and any error encountered.
func writeToSheetWithRateLimit(srv *sheets.Service, spreadsheetID, range_ string, vr *sheets.ValueRange) (*sheets.UpdateValuesResponse, error) {
	// Wait for a token from the rate limiter
	if err := sheetsLimiter.Wait(context.Background()); err != nil {
		return nil, err
	}

	// Proceed with the write operation
	return srv.Spreadsheets.Values.Update(spreadsheetID, range_, vr).ValueInputOption("USER_ENTERED").Do()
}

// NewStatsUpdater creates a new StatsUpdater. It decodes the service account key, creates a new Sheets service, and initializes the statRowMap.
// It also calls the CreateStatsSheet function to create the "Stats" sheet in the Google Sheets document if it doesn't already exist.
// Then it writes the provided stat names to the "Stats" sheet.
func NewStatsUpdater(spreadsheetID string, serviceAccountKey string, statNames []string) (*StatsUpdater, error) {
	key, err := base64.StdEncoding.DecodeString(serviceAccountKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding service account key: %v", err)
	}

	b, err := google.JWTConfigFromJSON(key, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	srv, err := sheets.NewService(context.Background(), option.WithHTTPClient(b.Client(context.Background())))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Sheets client: %v", err)
	}

	su := &StatsUpdater{
		srv:           srv,
		spreadsheetID: spreadsheetID,
		statRowMap:    make(map[string]int),
	}

	err = su.CreateStatsSheet()
	if err != nil {
		return nil, fmt.Errorf("failed to create Stats sheet: %v", err)
	}

	err = su.WriteStatNames(statNames)
	if err != nil {
		return nil, fmt.Errorf("failed to write stat names: %v", err)
	}

	return su, nil
}

// WriteStatNames clears the "Stats" sheet, writes the stat names to the "Stats" sheet, and updates the statRowMap.
func (su *StatsUpdater) WriteStatNames(statNames []string) error {
	err := su.ClearStatsSheet()
	if err != nil {
		return fmt.Errorf("failed to clear Stats sheet: %v", err)
	}

	for i, stat := range statNames {
		su.statRowMap[stat] = i + 1
		range_ := fmt.Sprintf("Stats!A%d", i+1)
		vr := &sheets.ValueRange{
			Values: [][]interface{}{{stat}},
		}
		_, err := writeToSheetWithRateLimit(su.srv, su.spreadsheetID, range_, vr)
		if err != nil {
			return fmt.Errorf("failed to write stat %q: %v", stat, err)
		}
	}

	return nil
}

// UpdateStats updates the value of a stat in the "Stats" sheet.
func (su *StatsUpdater) UpdateStats(statName string, value interface{}) error {
	row, ok := su.statRowMap[statName]
	if !ok {
		return fmt.Errorf("unknown stat: %s", statName)
	}
	range_ := fmt.Sprintf("Stats!B%d", row)
	vr := &sheets.ValueRange{
		Values: [][]interface{}{{value}},
	}
	_, err := writeToSheetWithRateLimit(su.srv, su.spreadsheetID, range_, vr)
	if err != nil {
		log.Printf("Error updating stats: %v", err)
		return fmt.Errorf("failed to update stat %q with value %v: %v", statName, value, err)
	}

	return nil
}

// ClearStatsSheet clears the "Stats" sheet in the Google Sheets document.
func (su *StatsUpdater) ClearStatsSheet() error {
	// Wait for a token from the rate limiter
	if err := sheetsLimiter.Wait(context.Background()); err != nil {
		return err
	}

	_, err := su.srv.Spreadsheets.Values.Clear(su.spreadsheetID, "Stats!A:B", &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return fmt.Errorf("failed to clear Stats sheet: %v", err)
	}

	return nil
}

// CreateStatsSheet creates a new "Stats" sheet in the Google Sheets document if it doesn't already exist.
func (su *StatsUpdater) CreateStatsSheet() error {
	// Wait for a token from the rate limiter
	if err := sheetsLimiter.Wait(context.Background()); err != nil {
		return err
	}

	spreadsheet, err := su.srv.Spreadsheets.Get(su.spreadsheetID).Do()
	if err != nil {
		return fmt.Errorf("failed to retrieve spreadsheet: %v", err)
	}

	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties.Title == "Stats" {
			return nil
		}
	}

	sheet := &sheets.Sheet{
		Properties: &sheets.SheetProperties{
			Title: "Stats",
		},
	}
	_, err = su.srv.Spreadsheets.BatchUpdate(su.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				AddSheet: &sheets.AddSheetRequest{
					Properties: sheet.Properties,
				},
			},
		},
	}).Do()
	if err != nil {
		return fmt.Errorf("failed to create Stats sheet: %v", err)
	}

	return nil
}
