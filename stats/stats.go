package stats

import (
	"context"
	"encoding/base64"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type StatsUpdater struct {
	srv           *sheets.Service
	spreadsheetID string
}

func NewStatsUpdater(spreadsheetID string, serviceAccountKey string) (*StatsUpdater, error) {
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

	return &StatsUpdater{
		srv:           srv,
		spreadsheetID: spreadsheetID,
	}, nil
}

func (su *StatsUpdater) CreateStatsSheet() error {
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

func (su *StatsUpdater) WriteStatNames(statNames []string) error {
	for i, stat := range statNames {
		range_ := fmt.Sprintf("Stats!A%d", i+1)
		vr := &sheets.ValueRange{
			Values: [][]interface{}{{stat}},
		}
		_, err := su.srv.Spreadsheets.Values.Update(su.spreadsheetID, range_, vr).ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			return fmt.Errorf("failed to write stat %q: %v", stat, err)
		}
	}

	return nil
}

func (su *StatsUpdater) UpdateStats(statName string, value interface{}) error {
	range_ := fmt.Sprintf("Stats!B%d", statName)
	vr := &sheets.ValueRange{
		Values: [][]interface{}{{value}},
	}
	_, err := su.srv.Spreadsheets.Values.Update(su.spreadsheetID, range_, vr).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return fmt.Errorf("failed to write value %q: %v", value, err)
	}

	return nil
}
