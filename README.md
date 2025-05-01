# 📆 gcalsync: Sync Your Google Calendars Like a Boss! 🚀

[![Go Version](https://img.shields.io/badge/go-1.16+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-0969da?style=flat-square&logo=opensource)](https://opensource.org/licenses/MIT)

Welcome to **gcalsync**, the ultimate tool for syncing your Google Calendars across multiple accounts!
Say goodbye to calendar conflicts and hello to seamless synchronization. 🎉

## 🌟 Features

-   🔄 Sync events from multiple Google Calendars, Microsoft Exchange, and CalDAV calendars (iCal, Nextcloud, etc.)
-   🚫 Create "blocker" events in other calendars to prevent double bookings
-   🗄️ Store access tokens and calendar data securely in a local SQLite database
-   🔒 Authenticate with Google and Microsoft using the OAuth2 flow for desktop apps
-   🌐 Support for CalDAV/iCal calendars through standard HTTP authentication
-   ☁️ Support for Microsoft Exchange with OKTA SSO integration
-   🧹 Easy way to cleanup calendars and remove all blocker events with a single command

## 📋 Prerequisites

-   Go 1.16 or higher
-   A Google Cloud Platform project with the Google Calendar API enabled (for Google Calendar)
-   OAuth2 credentials (client ID and client secret) for the Google desktop app flow
-   Azure App registration with Microsoft Graph API permissions (for Microsoft Calendar with OKTA SSO)

## 🚀 Getting Started

1. Clone the repository:

    ```
    git clone https://github.com/bobuk/gcalsync.git
    ```

2. Navigate to the project directory:

    ```
    cd gcalsync
    ```

3. Install the dependencies:

    ```
    go mod download
    ```

4. Create a `.gcalsync.toml` file in the project directory with your credentials:

    ```toml
    [general]
    disable_reminders = false              # Disable reminders for blocker events
    block_event_visibility = "private"     # Visibility of blocker events (private, public, or default)
    authorized_ports = [8080, 8081, 8082]  # Ports that can be used for OAuth callback
    
    [google]
    client_id = "your-client-id"           # Your OAuth2 client ID
    client_secret = "your-client-secret"   # Your OAuth2 client secret
    
    # Optional: For CalDAV/iCal calendar support
    [caldav_servers.work]
    server_url = "https://caldav.example.com/dav/"  # CalDAV server URL
    username = "your-username"                      # CalDAV username
    password = "your-password"                      # CalDAV password
    name = "Work Calendar"                          # Optional friendly name
    ```

    Don't forget to choose the appropriate OAuth2 consent screen settings and [add the necessary scopes](https://developers.google.com/identity/oauth2/web/guides/get-google-api-clientid) for the Google Calendar API, also double check that you are select "Desktop app" as application type.

    You can move the file to `~/.config/gcalsync/.gcalsync.toml` to avoid storing sensitive data in the project directory. In this case your database file will be created in `~/.config/gcalsync/` as well.

5. Build the executable:

    ```
    go build
    ```

6. Run the `gcalsync` command with the desired action:
    - To add a new calendar:
        ```
        ./gcalsync add
        ```
    - To sync calendars:
        ```
        ./gcalsync sync
        ```
    - To desync calendars:
        ```
        ./gcalsync desync
        ```
    - To list all calendars:
        ```
        ./gcalsync list
        ```

## 📚 Documentation

### 🆕 Adding a Calendar

To add a new calendar to sync, run the `gcalsync add` command. You will be prompted to enter:

1. The account name (a label to identify this calendar)
2. The provider type (google, caldav, or microsoft)
3. The calendar ID or URL:
   - For Google calendars: typically your email address or a specific calendar ID
   - For CalDAV calendars: the full URL to the calendar (e.g., https://caldav.example.com/dav/calendars/user/calendar-name/)
   - For Microsoft calendars: typically your email address or a specific calendar ID

For Google and Microsoft calendars with OAuth, the program will guide you through the authentication process and store the access token securely in the local database. For CalDAV calendars and Microsoft with basic auth, authentication is handled using the credentials specified in your config file.

### 🔄 Syncing Calendars

To sync your calendars, run the `gcalsync sync` command. The program will retrieve events from the specified calendars within the current and next month time window. It will create "blocker" events in other calendars to prevent double bookings and store the blocker event details in the local database.

### 🧹 Desyncing Calendars

To desync your calendars and remove all blocker events, run the `gcalsync desync` command. The program will retrieve the blocker event details from the local database and remove the corresponding events from the respective calendars.

### 📋 Listing Calendars

To list all calendars that have been added to the local database, run the `gcalsync list` command. The program will display the account name and calendar ID for each calendar.

### 🎗️ Disabling Reminders

By default blocker events will inherit your default Google Calendar reminder/alert settings (typically – 10 minutes before the event). If you *do not want* to receive reminders for the blocker events, you can disable them by setting the `disable_reminders` field to `true` in the `.gcalsync.toml` configuration file.

### 🕶️ Setting Block Event Visibility

By default blocker events will be created with the visibility set to "private". If you want to change the visibility of blocker events, you can set the `block_event_visibility` field to "public" or "default" in the `.gcalsync.toml` configuration file.

### ☁️ Microsoft Calendar with OKTA SSO

To set up Microsoft Calendar integration using OKTA SSO:

1. **Create an Azure App Registration**:
   - Go to the [Azure Portal](https://portal.azure.com)
   - Navigate to "Azure Active Directory" → "App registrations"
   - Click "New registration"
   - Enter a name (e.g., "GCalSync")
   - For "Supported account types", select "Accounts in this organizational directory only"
   - Add "http://localhost:8080" as a Web platform Redirect URI
   - Click "Register"

2. **Configure API Permissions**:
   - In your registered app, go to "API permissions"
   - Click "Add a permission"
   - Select "Microsoft Graph" → "Delegated permissions"
   - Add: Calendars.Read, Calendars.ReadWrite, User.Read
   - Click "Add permissions" then "Grant admin consent"

3. **Create Client Secret**:
   - Go to "Certificates & secrets"
   - Click "New client secret"
   - Add a description and select expiration period
   - Click "Add" and **immediately copy the secret value**

4. **Configure gcalsync**:
   - Add Microsoft configuration to your `.gcalsync.toml` file (see configuration section)
   - Use the `gcalsync add` command to add your Microsoft calendar

When you run the sync command, gcalsync will handle the OKTA SSO authentication flow and store the token securely in the database.

### Configuration File

The `.gcalsync.toml` configuration file is used to store OAuth2 credentials and general settings for the program. You can customize the settings to suit your preferences and needs. The file should be located in the project directory or `~/.config/gcalsync/` directory.

At a minimum, the configuration file should contain the following fields:

```toml
[general]
block_event_visibility = "private"    # Keep O_o event public or private
disable_reminders = true              # Set reminders on O_o events or not
verbosity_level = 1                   # How much chatter to spill out when running sync
authorized_ports = [3000, 3001, 3002] # Callback ports to listen to for OAuth token response

[google]
client_id = "your-client-id"          # Your Google app client ID
client_secret = "your-client-secret"  # Your Google app configuration secret

[caldav_servers.example]
server_url = "https://caldav.example.com/dav/" # CalDAV server URL
username = "your-username"                     # CalDAV username
password = "your-password"                     # CalDAV password
name = "Example CalDAV"                        # Optional friendly name

# Optional: For Microsoft Exchange with OAuth/OKTA SSO
[microsoft_servers.example]
server_url = "https://graph.microsoft.com/v1.0"
name = "Microsoft Exchange"
use_oauth = true
client_id = "your-azure-app-client-id"
client_secret = "your-azure-app-client-secret"
tenant_id = "your-tenant-id"
authority_url = "https://login.microsoftonline.com/your-tenant-id"
redirect_url = "http://localhost:8080"
```

#### 🔌 Configuration Parameters

- `[general]` section
  - `authorized_ports`: The application needs to start a temporary local server to receive the OAuth callback from Google. By default, it will try ports 8080, 8081, and 8082. You can customize these ports by setting the `authorized_ports` array in your configuration file. The application will try each port in order until it finds an available one. Make sure these ports are allowed by your firewall and not in use by other applications.
  - `block_event_visibility`: Defines whether you want to keep blocker events ("O_o") publicly visible or not. Possible values are `private` or `public`. If omitted -- `public` is used.
  - `disable_reminders`: Whether your blocker events should stay quite and **not** alert you. Possible values are `true` or `false`. default is `false`.
  - `verbosity_level`: How "chatty" you want the app to be 1..3 with 1 being mostly quite and 3 giving you full details of what it is doing.

- `[google]` section
  - `client_id`: Your Google app client ID
  - `client_secret`: Your Google app configuration secret

- `[caldav_servers.<name>]` section
  - `server_url`: Base URL of your CalDAV server
  - `username`: Username for CalDAV authentication
  - `password`: Password for CalDAV authentication
  - `name`: Optional friendly name for the CalDAV server
  
- `[microsoft_servers.<name>]` section
  - `server_url`: Microsoft Graph API URL (for OAuth) or Exchange ActiveSync URL
  - `name`: Optional friendly name for the Microsoft server
  - `use_oauth`: Set to true for OAuth authentication (OKTA SSO), false for basic authentication
  - `client_id`: Your Azure app client ID (required for OAuth)
  - `client_secret`: Your Azure app client secret (required for OAuth)
  - `tenant_id`: Your Azure tenant ID (required for OAuth)
  - `authority_url`: Authority URL for OAuth authentication (usually https://login.microsoftonline.com/your-tenant-id)
  - `redirect_url`: Redirect URL for OAuth callback
  - `username`: Username for basic authentication (only if use_oauth is false)
  - `password`: Password for basic authentication (only if use_oauth is false)

## 🤝 Contributing

Contributions are welcome! If you encounter any issues or have suggestions for improvement, please open an issue or submit a pull request. Let's make gcalsync even better together! 💪

## 📄 License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT). Feel free to use, modify, and distribute the code as you see fit. We hope you find it useful! 🌟

## 🙏 Acknowledgements

-   The terrible [Go](https://golang.org/) programming language
-   The [Google Calendar API](https://developers.google.com/calendar) for making this project almost impossible to implement
-   The [Microsoft Graph API](https://learn.microsoft.com/en-us/graph/overview) for being slightly more reasonable
-   The [OAuth2](https://oauth.net/2/) protocol for very missleading but secure authentication
-   The [SQLite](https://www.sqlite.org/) database for lightweight and efficient storage, the only one that added no pain
-   The [go-webdav](https://github.com/emersion/go-webdav) library for excellent WebDAV/CalDAV support
-   The [go-ical](https://github.com/emersion/go-ical) library for parsing and generating iCalendar data
-   The [Microsoft Authentication Library for Go](https://github.com/AzureAD/microsoft-authentication-library-for-go) for OAuth support with Azure AD
