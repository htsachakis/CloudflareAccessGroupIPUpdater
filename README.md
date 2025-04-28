# Cloudflare Access Group IP Updater
A Go CLI application that automatically updates your Cloudflare Access Group IP address when your public IP changes. This is useful for maintaining secure access to your resources when your IP address changes dynamically.

## Features

- Retrieves your current public IP address using ipify.org
- Gets your Cloudflare Access Group configuration using the Cloudflare API
- Compares your current IP with the one in your Cloudflare Access Group
- Updates the Access Group if the IP has changed
- Runs on a cron schedule you specify via environment variables
- Sends notifications when IP changes or on errors via Shoutrrr (supports Discord, Slack, Telegram, email, and more)
- Test notification feature to verify your notification setup
- Built with Go for efficient resource usage

## Requirements

- Go 1.18+ (for development)
- Docker (optional, for containerized deployment)
- Cloudflare account with [API access](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/)
- - Required "Access: Organizations, Identity Providers, and Groups Read/Write" Check [here](https://developers.cloudflare.com/api/resources/zero_trust/subresources/access/subresources/groups/methods/list/)

## Configuration

The application is configured via environment variables:

| Environment Variable | Description                                                                                | Required |
|----------------------|--------------------------------------------------------------------------------------------|----------|
| `ACCOUNTID`          | Your Cloudflare account ID                                                                 | Yes      |
| `RULEID`             | Your Cloudflare Access Group rule ID                                                       | Yes      |
| `CRON`               | Cron schedule for checking and updating the IP (e.g., `*/30 * * * *` for every 30 minutes) | Yes      |
| `AUTH_TOKEN`         | Your Cloudflare API Bearer token with appropriate permissions                              | Yes      |
| `NOTIFICATION_URL`   | Shoutrrr URL for notifications (see below for examples)                                    | No       |
| `TEST_NOTIFICATION`  | Set to "true" to send a test notification on startup                                       | No       |

### Notification URL Format

The `NOTIFICATION_URL` uses Shoutrrr's URL format. Here are some examples:

- Discord: `discord://token@channel`
- Telegram: `telegram://token@telegram?chats=@channel`
- Slack: `slack://token@channel`
- Email (SMTP): `smtp://username:password@host:port/?from=from@example.com&to=to@example.com`
- Microsoft Teams: `teams://token1/token2/token3`
- Pushover: `pushover://token@user/?devices=device1,device2`

For more details and examples, see the [Shoutrrr documentation](https://containrrr.dev/shoutrrr/v0.8/services/overview/).

## Getting Started

### Running Locally

1. Clone this repository:
   ```bash
   git clone https://github.com/htsachakis/CloudflareAccessGroupIPUpdater
   cd CloudflareAccessGroupIPUpdater
   ```

2. Copy the example environment file and edit it with your details:
   ```bash
   cp .env.example .env
   # Edit .env with your values
   ```

3. Build the application:
   ```bash
   go build -o cloudflare-access-group-ip-updater
   ```

4. Run the application:
   ```bash
   export $(cat .env | xargs) && ./cloudflare-access-group-ip-updater
   ```

### Using Docker

1. Build the Docker image:
   ```bash
   docker build -t cloudflare-access-group-ip-updater .
   ```

2. Run the container with your environment variables:
   ```bash
   docker run -d --name cloudflare-access-group-ip-updater \
     --env-file .env \
     cloudflare-access-group-ip-updater
   ```

### Using Docker Compose

1. Copy the example `.env` file and edit it with your details:
   ```bash
   cp .env.example .env
   # Edit .env with your values
   ```

2. Launch the service:
   ```bash
   docker-compose up -d
   ```

3. Check the logs:
   ```bash
   docker-compose logs -f
   ```

4. Stop the service when needed:
   ```bash
   docker-compose down
   ```

### Prebuild Docker Compose (Recommended)

1. Create a file `docker-compose.yml` .
2. Paste in the file:
   ```yml
   services:
      cloudflare-ip-updater:
        image: ghcr.io/htsachakis/cloudflare-access-group-ip-updater:latest
        container_name: cloudflare-ip-updater
        restart: unless-stopped
        environment:
          - ACCOUNTID=your_cloudflare_account_id
          - RULEID=your_cloudflare_rule_id
          - CRON=*/30 * * * *
          - AUTH_TOKEN=your_cloudflare_api_token
          #- NOTIFICATION_URL=
          #- TEST_NOTIFICATION=true
          #- NOTIFICATION_IDENTIFIER="Server Name"
        volumes:
          - /etc/timezone:/etc/timezone:ro
          - /etc/localtime:/etc/localtime:ro
        logging:
          driver: "json-file"
          options:
            max-size: "10m"
            max-file: "3"
   ```
3. Replace the environment variables with yours
4. Run the service:```docker-compose up -d```




## Cron Schedule Format

The CRON environment variable uses the standard cron format:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │                                   
│ │ │ │ │
│ │ │ │ │
* * * * *
```

Common examples:
- `*/5 * * * *` - Every 5 minutes
- `0 * * * *` - Every hour, at minute 0
- `0 0 * * *` - Every day at midnight

## Notifications

The application can send notifications in the following scenarios:

- When started (if TEST_NOTIFICATION is set to "true")
- When the IP is changed successfully
- When an error occurs (fetching IP, accessing Cloudflare API, etc.)
- When the application shuts down

### Notification Examples

Here are examples of notifications you'll receive:

- 🚀 Cloudflare IP Updater started—Test notification
- ✅ Initial IP set in Cloudflare Access Group: 203.0.113.1
- 🔄 IP Address Updated: 203.0.113.1 ➡️ 198.51.100.1
- ❌ Error getting current IP: connection refused
- ⏹️ Cloudflare IP Updater stopped

## License

MIT

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.