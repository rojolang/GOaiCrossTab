package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/cenkalti/backoff"
	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
	"github.com/rojolang/GOaiCrossTab/stats"
	"github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2/google"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ColumnVariable struct {
	ColumnNumber int
	VariableName string
}

type ChunkSettings struct {
	Name          string
	TriggerColumn []string
	SystemMessage string
	UserMessage   string
	Temperature   float32
	MaxTokens     int
	PromptColTo   string
}

var allSettings map[string]map[string]interface{}
var _ map[string]ChunkSettings
var gptSettingsByName map[string]ChunkSettings
var _ map[int]ColumnVariable
var columnNameByIndex map[int]string
var columnIndexByName map[string]int
var columnLetterByName map[string]string
var currentRow map[string]interface{}
var redisClient *redis.Client
var srv *sheets.Service
var prevState [][]interface{}
var sheetsLimiter = rate.NewLimiter(rate.Every(time.Minute/29), 29)
var gptLimiter = rate.NewLimiter(rate.Every(time.Minute/10), 10)
var gptSemaphore = make(chan struct{}, 10)
var sheetsSemaphore = make(chan struct{}, 29)
var cellMutexes map[string]*sync.Mutex
var spreadsheetID string
var errorCount int
var successfulCompletions int

var su *stats.StatsUpdater // Define global variable for stats updater

// setupEnvironment loads environment variables, creates the Google Sheets and Redis services,
// and initializes a map of mutexes for each cell in the Google Sheets.
// It also creates a stats updater, creates the "Stats" sheet, and writes the stat names.
// It handles any errors that occur during these operations by calling the handleError function,
// which increments the "Errors" counter and updates the "Errors" and "Last Error" stats.
// It returns an error if an error occurred while creating the mutexes.
func setupEnvironment() error {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		handleError(err) // Call handleError function instead of returning the error directly
		return nil
	}

	// Get spreadsheet ID from environment variable
	spreadsheetID = os.Getenv("SPREADSHEET_ID")

	// Decode service account key
	b, err := base64.StdEncoding.DecodeString(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		handleError(err) // Call handleError function instead of returning the error directly
		return nil
	}

	// Create JWT config from service account key
	conf, err := google.JWTConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		handleError(err) // Call handleError function instead of returning the error directly
		return nil
	}

	// Create new Sheets service
	tmpSrv, err := sheets.NewService(context.Background(), option.WithHTTPClient(conf.Client(context.Background())))
	if err != nil {
		handleError(err) // Call handleError function instead of returning the error directly
		return nil
	}
	srv = tmpSrv

	// Create new StatsUpdater
	su, err = stats.NewStatsUpdater(spreadsheetID, os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		handleError(err) // Call handleError function instead of returning the error directly
		return nil
	}

	// Get Redis configuration from environment variables
	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		handleError(err) // Call handleError function instead of returning the error directly
		return nil
	}

	// Create new Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Test Redis connection
	_, err = redisClient.Ping().Result()
	if err != nil {
		handleError(err) // Call handleError function instead of returning the error directly
		return nil
	}

	// Fetch the entire sheet's data
	resp, err := srv.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		handleError(err) // Call handleError function instead of returning the error directly
		return nil
	}

	// Get the number of rows and columns from the sheet's grid data
	numRows := int(resp.Sheets[0].Properties.GridProperties.RowCount)
	numCols := int(resp.Sheets[0].Properties.GridProperties.ColumnCount)

	// Initialize the cellMutexes map with a mutex for each cell in the sheet
	cellMutexes = make(map[string]*sync.Mutex)
	for i := 0; i < numRows; i++ {
		for j := 0; j < numCols; j++ {
			cellKey := fmt.Sprintf("%d:%d", i, j)
			cellMutexes[cellKey] = &sync.Mutex{}
		}
	}

	return nil
}

// handleError function increments the "Errors" counter and updates the "Errors" and "Last Error" stats.
// handleError function increments the "Errors" counter and updates the "Errors" and "Last Error" stats.
func handleError(err error) {
	// Increment the "Errors" counter
	errorCount++

	// If STATS is true, update the "Errors" and "Last Error" stats
	if statsEnabled, ok := allSettings["GLOBAL"]["STATS"].(bool); ok && statsEnabled {
		// Assume su is an instance of StatsUpdater from the stats.go file
		errUpdate := su.UpdateStats("Errors", errorCount)
		if errUpdate != nil {
			log.Printf("Error updating stats: %v", errUpdate)
		}

		errUpdate = su.UpdateStats("Last Error", err.Error())
		if errUpdate != nil {
			log.Printf("Error updating stats: %v", errUpdate)
		}
	}
}

func readSettings() error {
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, "Settings!A1:B1000").Do()
	if err != nil {
		return fmt.Errorf("unable to retrieve data from sheet: %v", err)
	}

	allSettings = make(map[string]map[string]interface{})
	allSettings["GLOBAL"] = make(map[string]interface{})

	gptSettingsByName = make(map[string]ChunkSettings)
	_ = make(map[int]ColumnVariable)

	for _, row := range resp.Values {
		if len(row) < 2 {
			continue
		}
		key, ok := row[0].(string)
		if !ok {
			return fmt.Errorf("error: key is not a string. It is a %T", row[0])
		}
		value := row[1]

		if !strings.HasPrefix(key, "VAR") {
			switch key {
			case "SHEET_REFRESH_FREQUENCY":
				if s, err := strconv.ParseFloat(value.(string), 64); err == nil {
					allSettings["GLOBAL"]["SHEET_REFRESH_FREQUENCY"] = s
				} else {
					log.Printf("Error: SHEET_REFRESH_FREQUENCY is not a float64. It is a %s", value)
				}
			case "SHEET_NEW_COLUMNS_FREQUENCY":
				if s, err := strconv.ParseFloat(value.(string), 64); err == nil {
					allSettings["GLOBAL"]["SHEET_NEW_COLUMNS_FREQUENCY"] = s
				} else {
					log.Printf("Error: SHEET_NEW_COLUMNS_FREQUENCY is not a float64. It is a %s", value)
				}
			case "GPT_RATE_LIMIT":
				if s, err := strconv.Atoi(value.(string)); err == nil {
					gptLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(s)), s)
				} else {
					log.Printf("Error: GPT_RATE_LIMIT is not an int. It is a %s", value)
				}
			case "SHEETS_RATE_LIMIT":
				if s, err := strconv.Atoi(value.(string)); err == nil {
					sheetsLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(s)), s)
				} else {
					log.Printf("Error: SHEETS_RATE_LIMIT is not an int. It is a %s", value)
				}
			case "STATS":
				if s, err := strconv.ParseBool(value.(string)); err == nil {
					allSettings["GLOBAL"]["STATS"] = s
				} else {
					log.Printf("Error: STATS is not a bool. It is a %s", value)
				}
			default:
				allSettings["GLOBAL"][key] = value
			}

		} else {
			splitIndex := strings.Index(key, "_")
			currentSettingsName := key[:splitIndex]
			varProp := key[splitIndex+1:]
			varValue, ok := value.(string)
			if !ok {
				return fmt.Errorf("error: varValue is not a string. It is a %T", value)
			}

			currentSettings, exists := gptSettingsByName[currentSettingsName]
			if !exists {
				currentSettings = ChunkSettings{Name: currentSettingsName}
			}

			switch varProp {
			case "TRIGGER_COL":
				splitValues := strings.Split(varValue, ",")
				for i, val := range splitValues {
					splitValues[i] = strings.TrimSpace(val)
				}
				currentSettings.TriggerColumn = splitValues
			case "SYSTEM_MESSAGE":
				currentSettings.SystemMessage = varValue
			case "USER_MESSAGE":
				currentSettings.UserMessage = varValue
			case "TEMP":
				if temp, err := strconv.ParseFloat(varValue, 32); err == nil {
					currentSettings.Temperature = float32(temp)
				} else {
					return fmt.Errorf("error: TEMP is not a float32. It is a %s", varValue)
				}
			case "MAX_TOKENS":
				if maxTokens, err := strconv.Atoi(varValue); err == nil {
					currentSettings.MaxTokens = maxTokens
				} else {
					return fmt.Errorf("error: MAX_TOKENS is not an int. It is a %s", varValue)
				}
			case "PROMPT_COL_TO":
				currentSettings.PromptColTo = varValue
			}

			gptSettingsByName[currentSettingsName] = currentSettings
		}
	}
	return nil
}

// detectChanges compares the current state with the previous state and logs any changes.
// It updates the previous state in Redis and processes any detected changes.
// It returns an error if an error occurred.
func detectChanges(currentRows [][]interface{}, shouldCheckForNewColumns bool) error {
	columnNameByIndex = make(map[int]string)
	columnIndexByName = make(map[string]int)
	columnLetterByName = make(map[string]string)

	for i := range currentRows[0] {
		columnName, ok := currentRows[0][i].(string)
		if !ok {
			log.Printf("Error: columnName is not a string. It is a %T", currentRows[0][i])
			continue
		}
		columnNameByIndex[i] = columnName
		columnIndexByName[columnName] = i
		columnLetterByName[columnName] = getExcelColumnName(i + 1)
	}

	for rowIndex := range currentRows {
		if rowIndex == 0 {
			continue
		}

		currentRow = make(map[string]interface{})

		for columnIndex := range currentRows[rowIndex] {
			currentRow["RowIndex"] = rowIndex
			currentRow[columnNameByIndex[columnIndex]] = currentRows[rowIndex][columnIndex]
		}

		for _, columnName := range columnNameByIndex {
			if _, ok := currentRow[columnName]; !ok {
				currentRow[columnName] = ""
			}
		}

		for _, gptSettings := range gptSettingsByName {
			if len(gptSettings.UserMessage) == 0 || len(gptSettings.SystemMessage) == 0 {
				continue
			}
			if gptSettings.Temperature == 0 || gptSettings.MaxTokens == 0 {
				continue
			}
			if len(gptSettings.PromptColTo) == 0 {
				continue
			}
			if len(gptSettings.TriggerColumn) == 0 {
				continue
			}

			rowHadTriggerColumnValues := true
			rowHadChangedTriggerColumnsCount := 0
			rowMissingNewColumns := false

			for _, triggerColumn := range gptSettings.TriggerColumn {
				if currentRow[triggerColumn] == nil || currentRow[triggerColumn] == "" {
					rowHadTriggerColumnValues = false
					break
				}

				if checkIfValueChangedInCache(currentRow, triggerColumn) {
					rowHadChangedTriggerColumnsCount++
				}
			}

			if shouldCheckForNewColumns {
				newColumnValue := currentRow[gptSettings.PromptColTo]
				if newColumnValue == nil || newColumnValue == "" {
					if rowHadTriggerColumnValues {
						rowMissingNewColumns = true
					}
				}
			}

			if rowMissingNewColumns || (rowHadTriggerColumnValues && rowHadChangedTriggerColumnsCount == len(gptSettings.TriggerColumn)) {
				if rowMissingNewColumns {
					log.Printf("Row #%d is missing value in new column '%s'\n", currentRow["RowIndex"], gptSettings.PromptColTo)
				} else {
					log.Printf("Row #%d change triggered gptSettings '%s'\n", currentRow["RowIndex"], gptSettings.Name)
				}

				err := runGptSettingsOnRowWithSemaphore(currentRow, gptSettings)
				if err != nil {
					log.Printf("Error running GPT settings on row: %v", err)
					continue
				}
			}
		}
	}
	return nil
}

// getExcelColumnName function converts a column number to an Excel column name.
// It returns the Excel column name.
func getExcelColumnName(columnNumber int) string {
	columnName := ""
	for columnNumber > 0 {
		columnNumber--
		columnName = string(rune('A'+columnNumber%26)) + columnName
		columnNumber /= 26
	}
	return columnName
}

// checkIfValueChangedInCache function checks if a value has changed in the Redis cache.
// It returns true if the value has changed, false otherwise.
func checkIfValueChangedInCache(row map[string]interface{}, columnName string) bool {
	if row[columnName] == nil || row[columnName] == "" {
		return false
	}

	redisKey := fmt.Sprintf("cell:%d:%d", row["RowIndex"], columnIndexByName[columnName])

	prevValue, err := redisClient.Get(redisKey).Result()
	if err == redis.Nil {
		// The key does not exist in Redis, which means the cell's value has not been cached yet.
		// Cache the value and return true to indicate that the value has "changed".
		err = redisClient.Set(redisKey, row[columnName], 0).Err()
		if err != nil {
			log.Printf("Error setting value in Redis: %v", err)
		}
		return true
	} else if err != nil {
		log.Printf("Error getting value from Redis: %v", err)
		return false
	}

	if prevValue == row[columnName] {
		return false
	}

	log.Printf("Row #%d (%v) has changed from %v to %v\n", row["RowIndex"], columnName, prevValue, row[columnName])

	err = redisClient.Set(redisKey, row[columnName], 0).Err()
	if err != nil {
		log.Printf("Error setting value in Redis: %v", err)
	}

	return true
}

// replaceTokens function replaces tokens in a message with their corresponding values from a row.
// It returns the message with tokens replaced.
func replaceTokens(message string, currentRow map[string]interface{}) string {
	for key, value := range currentRow {
		token := "{" + key + "}"
		if strings.Contains(message, token) {
			message = strings.Replace(message, token, fmt.Sprint(value), -1)
		}
	}
	return message
}

// Mutex for synchronizing access to critical sections of code

// runGptSettingsOnRow processes the GPT settings on a row.
// It fetches the GPT response, updates the Google Sheet with the response, and logs any errors.
// It returns an error if an error occurred.
func runGptSettingsOnRow(row map[string]interface{}, gptSettings ChunkSettings) error {
	if err := gptLimiter.Wait(context.Background()); err != nil {
		log.Printf("[GPT] rate limit error: %v", err)
		return err
	}

	destinationColumnName := gptSettings.PromptColTo
	destinationColumnLetter := columnLetterByName[destinationColumnName]
	rowIndex, ok := row["RowIndex"].(int)
	if !ok {
		log.Printf("Error: RowIndex is not an integer")
		return fmt.Errorf("error: RowIndex is not an integer")
	}
	destinationRange := fmt.Sprintf("%v%d", destinationColumnLetter, rowIndex+1)

	vr := &sheets.ValueRange{
		Values: [][]interface{}{{""}},
	}
	updateSheetWithSemaphore(spreadsheetID, destinationRange, vr)

	systemMessage := replaceTokens(gptSettings.SystemMessage, row)
	userMessage := replaceTokens(gptSettings.UserMessage, row)

	client := openai.NewClient(os.Getenv("OPENAI_SECRET_KEY"))
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemMessage,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userMessage,
				},
			},
			MaxTokens:   gptSettings.MaxTokens,
			Temperature: float32(gptSettings.Temperature),
		},
	)

	if err != nil {
		log.Printf("[ERROR] getting GPT response: %v", err)
		return err
	}

	if len(resp.Choices) <= 0 {
		log.Printf("No choices in the response")
		return fmt.Errorf("no choices in the response")
	}

	output := resp.Choices[0].Message.Content
	vr = &sheets.ValueRange{
		Values: [][]interface{}{{output}},
	}
	updateSheetWithSemaphore(spreadsheetID, destinationRange, vr)
	log.Printf("Updated row #%v (%s) with value %s\n", rowIndex, destinationColumnName, output)
	// If STATS is true, update the "Successful Completions" stat
	if statsEnabled, ok := allSettings["GLOBAL"]["STATS"].(bool); ok && statsEnabled {
		// Assume su is an instance of StatsUpdater from the stats.go file
		// Assume successfulCompletions is a counter for the number of successful completions
		successfulCompletions++
		err := su.UpdateStats("Successful Completions", successfulCompletions)
		if err != nil {
			log.Printf("Error updating stats: %v", err)
		}
	}

	return nil
}

// runGptSettingsOnRowWithSemaphore is a function that runs the GPT settings on a row with a semaphore for rate limiting.
// It first acquires a token from the gptSemaphore, then launches a goroutine to run the GPT settings on the row.
// The goroutine first checks if a mutex exists for the cell. If it doesn't, it creates a new mutex and stores it in the map.
// It then locks the mutex for the cell to ensure exclusive access to the cell.
// It then calls the runGptSettingsOnRow function to process the GPT settings on the row.
// After the GPT settings have been processed, it unlocks the mutex for the cell to allow other goroutines to access the cell.
// If an error occurs while processing the GPT settings, it logs the error.
// Finally, it releases the token back to the gptSemaphore.
func runGptSettingsOnRowWithSemaphore(row map[string]interface{}, gptSettings ChunkSettings) error {
	gptSemaphore <- struct{}{}
	go func() {
		defer func() { <-gptSemaphore }()
		cellKey := fmt.Sprintf("%d:%d", row["RowIndex"], columnIndexByName[gptSettings.PromptColTo])
		mutex, ok := cellMutexes[cellKey]
		if !ok {
			mutex = &sync.Mutex{}
			cellMutexes[cellKey] = mutex
		}
		mutex.Lock()
		err := runGptSettingsOnRow(row, gptSettings)
		mutex.Unlock()
		if err != nil {
			log.Printf("Error running GPT settings on row: %v", err)
		}
	}()
	return nil
}

// updateSheetWithSemaphore is a function that updates a Google Sheet with a semaphore for rate limiting.
// It first acquires a token from the sheetsSemaphore, then launches a goroutine to update the Google Sheet.
// The goroutine first checks if a mutex exists for the cell. If it doesn't, it creates a new mutex and stores it in the map.
// It then locks the mutex for the cell to ensure exclusive access to the cell.
// It then calls the srv.Spreadsheets.Values.Update function to update the Google Sheet.
// If the destination range is invalid, it adds a new row or column to the cellMutexes map.
// After the Google Sheet has been updated, it unlocks the mutex for the cell to allow other goroutines to access the cell.
// If an error occurs while updating the Google Sheet, it logs the error.
// Finally, it releases the token back to the sheetsSemaphore.
func updateSheetWithSemaphore(spreadsheetID, destinationRange string, vr *sheets.ValueRange) {
	sheetsSemaphore <- struct{}{}
	go func() {
		defer func() { <-sheetsSemaphore }()

		// Parse the rowIndex and columnIndex from the destinationRange
		reg := regexp.MustCompile(`([A-Za-z]+)([0-9]+)`)
		matches := reg.FindStringSubmatch(destinationRange)
		if len(matches) < 3 {
			log.Printf("Invalid destination range: %s. Skipping update.", destinationRange)
			return
		}

		// Convert column letter to index
		columnIndex := columnLetterToIndex(matches[1])
		rowIndex, err := strconv.Atoi(matches[2])
		if err != nil {
			log.Printf("Error parsing rowIndex: %v", err)
			return
		}
		rowIndex-- // adjust to 0-indexed

		cellKey := fmt.Sprintf("%d:%d", rowIndex, columnIndex)
		mutex, ok := cellMutexes[cellKey]
		if !ok {
			mutex = &sync.Mutex{}
			cellMutexes[cellKey] = mutex
		}
		mutex.Lock()
		operation := func() error {
			err := sheetsLimiter.Wait(context.Background())
			if err != nil {
				log.Printf("[Sheets] rate limit error: %v", err)
				return err
			}
			_, err = srv.Spreadsheets.Values.Update(spreadsheetID, destinationRange, vr).ValueInputOption("USER_ENTERED").Do()
			if err != nil {
				log.Printf("Error updating Google Sheet: %v", err)
				return err
			}
			return nil
		}
		expBackOff := backoff.NewExponentialBackOff()
		err = backoff.Retry(operation, expBackOff)
		mutex.Unlock()
		if err != nil {
			return
		}
	}()
}

// columnLetterToIndex converts a column letter to an index.
func columnLetterToIndex(columnLetter string) int {
	columnIndex := 0
	multiplier := 1
	for i := len(columnLetter) - 1; i >= 0; i-- {
		columnIndex += int(columnLetter[i]-'A'+1) * multiplier
		multiplier *= 26
	}
	return columnIndex - 1 // adjust to 0-indexed
}

// runMainLoop runs the main loop of the program. It fetches the values from the Google Sheet,
// detects any changes, updates the previous state, and sleeps for the specified refresh frequency
// before fetching the values again. It also updates the "Total Rows Processed" stat if the STATS setting is true.
// It handles any errors that occur during these operations by calling the handleError function,
// which increments the "Errors" counter and updates the "Errors" and "Last Error" stats.
// It returns an error if an error occurred while sleeping.
func runMainLoop() error {
	lastColumnCheck := time.Now().Add(-1 * time.Hour)
	lastReadSettings := time.Now().Add(-1 * time.Hour)

	// Define a counter for the total number of rows processed
	totalRowsProcessed := 0

	if prevState == nil {
		prevState = make([][]interface{}, 0)
	}

	for {
		if time.Since(lastReadSettings).Seconds() >= 15 {
			err := readSettings()
			if err != nil {
				log.Printf("Error reading settings: %v", err)
				continue
			}
			lastReadSettings = time.Now()
		}

		resp, err := getSheetValuesWithSemaphore(spreadsheetID, allSettings["GLOBAL"]["SHEET_NAME"].(string))
		if err != nil {
			handleError(err) // Call handleError function instead of logging the error directly
			continue
		}

		// Increment the total number of rows processed by the number of rows
		totalRowsProcessed += len(resp.Values)

		// If STATS is true, update the "Total Rows Processed" stat
		if statsEnabled, ok := allSettings["GLOBAL"]["STATS"].(bool); ok && statsEnabled {
			err := su.UpdateStats("Total Rows Processed", totalRowsProcessed)
			if err != nil {
				handleError(err) // Call handleError function instead of logging the error directly
			}
		}

		if prevState == nil {
			for i := range resp.Values {
				for j := range resp.Values[i] {
					value, ok := resp.Values[i][j].(string)
					if !ok {
						handleError(fmt.Errorf("error: value is not a string. It is a %T", resp.Values[i][j])) // Call handleError function instead of logging the error directly
						continue
					}
					err := redisClient.Set(fmt.Sprintf("cell:%d:%d", i, j), value, 0).Err()
					if err != nil {
						handleError(err) // Call handleError function instead of logging the error directly
						continue
					}
				}
			}
			prevState = resp.Values
			continue
		}

		shouldCheckForNewColumns := false
		newColumnsFreq, ok := allSettings["GLOBAL"]["SHEET_NEW_COLUMNS_FREQUENCY"].(float64)
		if !ok {
			handleError(fmt.Errorf("error: SHEET_NEW_COLUMNS_FREQUENCY in allSettings is not a float value")) // Call handleError function instead of logging the error directly
			continue
		}
		if time.Since(lastColumnCheck).Seconds() >= newColumnsFreq {
			shouldCheckForNewColumns = true
			lastColumnCheck = time.Now()
		}

		err = detectChanges(resp.Values, shouldCheckForNewColumns)
		if err != nil {
			handleError(err) // Call handleError function instead of logging the error directly
			continue
		}
		prevState = resp.Values

		sleepFreq, ok := allSettings["GLOBAL"]["SHEET_REFRESH_FREQUENCY"].(float64)
		if !ok {
			handleError(fmt.Errorf("error: SHEET_REFRESH_FREQUENCY in allSettings is not a float value")) // Call handleError function instead of logging the error directly
			continue
		}
		time.Sleep(time.Duration(sleepFreq * float64(time.Second)))
	}
	return nil
}

// getSheetValuesWithSemaphore is a wrapper function for srv.Spreadsheets.Values.Get that uses a semaphore for rate limiting.
func getSheetValuesWithSemaphore(spreadsheetID, sheetName string) (*sheets.ValueRange, error) {
	sheetsSemaphore <- struct{}{}
	respCh := make(chan *sheets.ValueRange, 1)
	errCh := make(chan error, 1)
	go func() {
		defer func() { <-sheetsSemaphore }()
		resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, sheetName).Do()
		if err != nil {
			errCh <- fmt.Errorf("error getting sheet values: %v", err)
			return
		}
		respCh <- resp
		close(respCh)
		close(errCh)
	}()
	resp := <-respCh
	err := <-errCh
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// the main function is the entry point of the program.
// it loads the environment variables, creates the Google Sheets and Redis services,
// fetches the settings from the Google Sheet, stores the prompt settings,
// and runs the main loop of the program.
func main() {

	err := setupEnvironment()
	if err != nil {
		log.Fatalf("Error setting up environment: %v", err)
	}
	err = readSettings()
	if err != nil {
		log.Fatalf("Error reading settings: %v", err)
	}

	err = runMainLoop()
	if err != nil {
		log.Fatalf("Error running main loop: %v", err)
	}
}
