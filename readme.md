## GOaiCrossTab:
### Your GoLang, Google Sheets, and OpenAI Integration Tool 🧪🔬

GOaiCrossTab is a powerful Golang package that creates a bridge between Google Sheets and OpenAI's GPT-4 model, transforming your spreadsheet data into meaningful insights. It fetches data from Google Sheets, processes the data using OpenAI's GPT-4 model, and writes the AI-generated responses back to Google Sheets. It also uses Redis for caching data to improve performance and uses rate limiters and semaphores to handle rate limits imposed by the Google Sheets API and the OpenAI API.

### Dependencies 🧩

GOaiCrossTab relies on several dependencies:

- Github.com/cenkalti/backoff
- Github.com/go-redis/redis
- Github.com/joho/godotenv
- Github.com/sashabaranov/go-openai
- Golang.org/x/oauth2/google
- Golang.org/x/time/rate
- Google.golang.org/api/option
- Google.golang.org/api/sheets/v4

### Google Sheets Settings 📊

In your Google Sheets, you will need to add a settings tab with the following variables. You can add as many variables as you want, and the program will iterate over them:

- SHEET_NAME: The name of the sheet to fetch data from.
- SHEET_REFRESH_FREQUENCY: The frequency (in seconds) at which the Google Sheet is refreshed.
- SHEET_NEW_COLUMNS_FREQUENCY: The frequency (in seconds) at which new columns are checked in the Google Sheet.
- MODEL: The OpenAI model to use (e.g., "GPT4").
- MAX_REQUEST_PER_MINUTE: The maximum number of requests per minute to send to the OpenAI API.
- SHEETS_RATE_LIMIT: The rate limit for the Google Sheets API.
- GPT_RATE_LIMIT: The rate limit for the OpenAI API.
- STATS: TRUE or FALSE (if you want stats to show in a new tab called stats)
Each variable (e.g., VAR0, VAR1, etc.) has the following settings:

- TRIGGER_COL: The column in the Google Sheet that triggers the variable.
- SYSTEM_MESSAGE: The system message to send to the OpenAI model.
- USER_MESSAGE: The user message to send to the OpenAI model.
- TEMP: The temperature to use for the OpenAI model.
- MAX_TOKENS: The maximum number of tokens for the OpenAI model to generate.
- PROMPT_COL_TO: The column in the Google Sheet to write the AI-generated response to.

Here is an example of how you might set upcs.google.com/spreadsheets/d/1cXmc20GjkvkEiw7WH_zLdUev_vS14mHm-RU0Zm09Qtg/) 
### How to Run 🛠️
To run GOaiCrossTab, you will need to set up your environment variables. Here is an example `.env` file:

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


### Docker Deployment 🐳

GOaiCrossTab is packaged as a Docker image, so you can easily run it on any platform that supports Docker. Here is how to run it:

1. Install Docker on your machine. You can follow the official Docker installation guide for your specific operating system: [Docker Installation Guide](https://docs.docker.com/get-docker/)

2. Install Docker Compose on your machine. You can follow the official Docker Compose installation guide for your specific operating system: [Docker Compose Installation Guide](https://docs.docker.com/compose/install/)

3. Clone the GOaiCrossTab repository:

```bash
git clone https://github.com/rojolang/GOaiCrossTab.git
cd GOaiCrossTab
cp example.env .env
```
after this then fill in the values in the .env file 
The google service account key is a json file that you can get from the google cloud console but it needs to be converted to a base64 string

To Automatically Run This on Linux or Mac

```bash
read -p "Enter the path to your service account key: " key_path
echo "GOOGLE_APPLICATION_CREDENTIALS=$(base64 -i $key_path)" >> .env
nano .env
```
For Windows
```bash
$key_path = Read-Host -Prompt "Enter the path to your service account key"
$encoded_key = [Convert]::ToBase64String([IO.File]::ReadAllBytes($key_path))
"GOOGLE_APPLICATION_CREDENTIALS=$encoded_key" | Out-File -Append .env
notepad.exe .env
```

Or you manually convert the json file to base64 and then copy the contents of the file to the .env file

4. Run the following command to start the application:

```bash
docker-compose up -d
```
5. Run the following command to view the logs:

```bash
docker-compose logs -f
```
6. Run the following command to stop the application:

```bash
docker-compose down
```

### Docker Compose 🐳

The easiest way to run GOaiCrossTab is to use Docker Compose. Here is an example docker-compose.yml file:
1. Create a `docker-compose.yml` file in the root directory of your project.
2. Copy and paste the provided Docker Compose configuration into your `docker-compose.yml` file.

   ```YAML
   version: '3'
   services:
   app:
   build:
   context: .
   dockerfile: Dockerfile
   volumes:
    - .:/go/src/app
      ports:
    - "8080:8080"
      environment:
    - GOOGLE_APPLICATION_CREDENTIALS
    - OPENAI_SECRET_KEY
    - SPREADSHEET_ID
    - REDIS_PASSWORD
    - REDIS_ADDR
    - REDIS_DB
      depends_on:
    - redis
      redis:
      image: "redis:alpine"
      environment:
    - REDIS_PASSWORD
      ports:
    - "6379:6379"
      ```
####
3. This configuration defines two services: the Go application (`app`) and a Redis database (`redis`). 
####
4. The `app` service is built using the Dockerfile in the current directory. It maps the current directory on the host to `/go/src/app` inside the Docker container and forwards port `8080` from the container to the host.
####
5. The `redis` service uses the official Redis image from Docker Hub and forwards port `6379` from the container to the host.
####
6. The environment variables are defined in the `.env` file. Make sure you have this file set up with the correct values.
####
7. Save the `docker-compose.yml` file.
####
8. Open a terminal and navigate to the directory containing your `docker-compose.yml` file.
####
9. Run `docker-compose up -d` to start the services. Docker Compose will automatically build the `app` image before starting the services.
####
10. You can check the status of your services by running `docker-compose ps`. If everything is set up correctly, you should see your services listed as `Up`.

Remember to replace the placeholders in the environment variables with your actual values.

### Run Locally 🏃‍♂️

If you want to run the application locally without Docker, you can do so by following these steps:
####
1. Install Golang on your machine. You can follow the official Golang installation guide for your specific operating system: [Golang Installation Guide](https://go.dev/doc/install)
####
2. Clone the GOaiCrossTab repository:
####
```BASH
git clone https://github.com/rojolang/GOaiCrossTab.git
cd GOaiCrossTab
cp example.env .env
```
####
3. Fill in the values in the .env file as described in the Docker Deployment section.
####
4. Install the dependencies:
####
```BASH
go mod download
```
####
5. Run the application:
####
```BASH
go run main.go
```
####
6. The application should now be running on your local machine.

## The Brains 👥

Special thanks to:

- #### [@gluebag](https://github.com/gluebag)
- #### [@haitestucodes](https://github.com/haitestucodes)
###
> ##  :rocket:  **New developments @ [CNTRL.ai](https://cntrl.ai)** :rocket:

>#### Connect with us on Social Media 
>[![Twitter](https://img.icons8.com/color/48/000000/twitter--v1.png)](https://twitter.com/CNTRLAI) 
>[![Instagram](https://img.icons8.com/color/48/000000/instagram-new--v1.png)](https://www.instagram.com/cntrl.ai/) 
>[![YouTube](https://img.icons8.com/color/48/000000/youtube-play.png)](https://www.youtube.com/@CNTRLai) 
>[![TikTok](https://img.icons8.com/color/48/000000/tiktok.png)](https://www.tiktok.com/@cntrl.ai) 
>[![Facebook](https://img.icons8.com/color/48/000000/facebook.png)](https://facebook.com/cntrl.ai) 