# AI CrossTab: Your GoLang, Google Sheets, and OpenAI Integration Tool 🧪🔬

AI CrossTab is a powerful Golang package that creates a bridge between Google Sheets and OpenAI's GPT-4 model, transforming your spreadsheet data into meaningful insights. It fetches data from Google Sheets, processes the data using OpenAI's GPT-4 model, and writes the AI-generated responses back to Google Sheets. It also uses Redis for caching data to improve performance and uses rate limiters and semaphores to handle rate limits imposed by the Google Sheets API and the OpenAI API.

## Dependencies 🧩

AI CrossTab relies on several dependencies:

- Github.com/cenkalti/backoff
- Github.com/go-redis/redis
- Github.com/joho/godotenv
- Github.com/sashabaranov/go-openai
- Golang.org/x/oauth2/google
- Golang.org/x/time/rate
- Google.golang.org/api/option
- Google.golang.org/api/sheets/v4

## Google Sheets Settings 📊

In your Google Sheets, you will need to add a settings tab with the following variables. You can add as many variables as you want, and the program will iterate over them:

- SHEET_NAME: The name of the sheet to fetch data from.
- SHEET_REFRESH_FREQUENCY: The frequency (in seconds) at which the Google Sheet is refreshed.
- SHEET_NEW_COLUMNS_FREQUENCY: The frequency (in seconds) at which new columns are checked in the Google Sheet.
- MODEL: The OpenAI model to use (e.g., "GPT4").
- MAX_REQUEST_PER_MINUTE: The maximum number of requests per minute to send to the OpenAI API.
- SHEETS_RATE_LIMIT: The rate limit for the Google Sheets API.
- GPT_RATE_LIMIT: The rate limit for the OpenAI API.

Each variable (e.g., VAR0, VAR1, etc.) has the following settings:

- TRIGGER_COL: The column in the Google Sheet that triggers the variable.
- SYSTEM_MESSAGE: The system message to send to the OpenAI model.
- USER_MESSAGE: The user message to send to the OpenAI model.
- TEMP: The temperature to use for the OpenAI model.
- MAX_TOKENS: The maximum number of tokens for the OpenAI model to generate.
- PROMPT_COL_TO: The column in the Google Sheet to write the AI-generated response to.

Here is an example of how you might set up your Google Sheet: [Google Sheets Example](https://docs.google.com/spreadsheets/d/1gSA3coyBGv0MsILhSTtCBg5kvOr66GsPLpcYNQz3FY0/edit?usp=sharing)

## How to Run 🛠️

To run AI CrossTab, you will need to set up your environment variables. Here is an example `.env` file:

```bash
GOOGLE_APPLICATION_CREDENTIALS=[Your base64 encoded Google service account key]
OPENAI_SECRET_KEY=[Your OpenAI secret key]
SPREADSHEET_ID=[Your Google Sheets ID]
REDIS_ADDR=[localhost:6379]
REDIS_PASSWORD=[redis_password
REDIS_DB=[0]
```

To convert your Google service account key to base64, you can use the `base64` command in Linux or macOS:

```bash
base64 -i [PATH_TO_YOUR_KEY.json] -o key.txt
```

Add the service account to your Google Sheet by sharing it with the `client_email` in the JSON key.

Then, copy the contents of `key.txt` to your `.env` file.
Included is an example.env file that you can rename to .env and fill in the values.


## Docker Deployment 🐳

AI CrossTab is packaged as a Docker image, so you can easily run it on any platform that supports Docker. Here is how to run it:

1. Install Docker on your machine. You can follow the official Docker installation guide for your specific operating system: [Docker Installation Guide](https://docs.docker.com/get-docker/)
2. Pull the AI CrossTab Docker image: `docker pull [IMAGE_NAME]`
3. Run the Docker image: `docker run -d -p 8080:8080 [IMAGE_NAME]`
