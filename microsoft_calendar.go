package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	"github.com/emersion/go-webdav/microsoft"
	msgraph "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
	"golang.org/x/oauth2"
)

type MicrosoftCalendarProvider struct {
	client      *microsoft.Client
	graphClient *msgraph.GraphServiceClient
	ctx         context.Context
	serverURL   string
	useOAuth    bool
	config      MicrosoftConfig
	db          *sql.DB
	accountName string
}

// NewMicrosoftCalendarProvider creates a new Microsoft Calendar provider
func NewMicrosoftCalendarProvider(ctx context.Context, config MicrosoftConfig, db *sql.DB, accountName string) (*MicrosoftCalendarProvider, error) {
	var msClient *microsoft.Client
	var graphClient *msgraph.GraphServiceClient
	var err error

	if config.UseOAuth {
		// OAuth authentication for Microsoft Graph API
		graphClient, err = createGraphClient(ctx, config, db, accountName)
		if err != nil {
			return nil, fmt.Errorf("failed to create Microsoft Graph client: %w", err)
		}
	} else {
		// Basic authentication for Exchange ActiveSync
		httpClient := &http.Client{
			Transport: &microsoft.BasicAuthTransport{
				Username: config.Username,
				Password: config.Password,
			},
		}
		
		// Create Microsoft Exchange ActiveSync client
		msClient = microsoft.NewClient(httpClient, config.ServerURL)

		// Test connection by trying to connect to the server
		err = msClient.Ping(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Microsoft Exchange server: %w", err)
		}
	}

	return &MicrosoftCalendarProvider{
		client:      msClient,
		graphClient: graphClient,
		ctx:         ctx,
		serverURL:   config.ServerURL,
		useOAuth:    config.UseOAuth,
		config:      config,
		db:          db,
		accountName: accountName,
	}, nil
}

// createGraphClient creates a Microsoft Graph API client using OAuth
func createGraphClient(ctx context.Context, config MicrosoftConfig, db *sql.DB, accountName string) (*msgraph.GraphServiceClient, error) {
	var token *oauth2.Token
	
	// Try to get token from database
	var tokenJSON []byte
	err := db.QueryRow("SELECT token FROM microsoft_tokens WHERE account_name = ?", accountName).Scan(&tokenJSON)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("error retrieving token from database: %w", err)
	}

	if err == nil {
		// Token found in database
		err = json.Unmarshal(tokenJSON, &token)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling token: %w", err)
		}
	} else {
		// No token found, get a new one
		token, err = getMicrosoftTokenFromWeb(config)
		if err != nil {
			return nil, fmt.Errorf("error getting token from web: %w", err)
		}
		
		// Save the token to database
		saveMicrosoftToken(db, accountName, token)
	}

	// Create credential
	cred, err := confidential.NewCredFromSecret(config.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("error creating credential: %w", err)
	}

	// Create confidential client application
	app, err := confidential.New(config.ClientID, cred, confidential.WithAuthority(config.AuthorityURL))
	if err != nil {
		return nil, fmt.Errorf("error creating confidential client: %w", err)
	}

	// Create a custom token provider that uses our token
	tokenProvider := func(ctx context.Context, scopes []string) (string, error) {
		// Check if token is expired and refresh if necessary
		if token.Expiry.Before(time.Now()) {
			// Get a new token
			newToken, err := getMicrosoftTokenFromWeb(config)
			if err != nil {
				return "", fmt.Errorf("error refreshing token: %w", err)
			}
			token = newToken
			saveMicrosoftToken(db, accountName, token)
		}
		return token.AccessToken, nil
	}

	// Create Graph client
	graphClient, err := msgraph.NewGraphServiceClientWithCredentials(tokenProvider, []string{"https://graph.microsoft.com/.default"})
	if err != nil {
		return nil, fmt.Errorf("error creating graph client: %w", err)
	}

	return graphClient, nil
}

// getMicrosoftTokenFromWeb gets a new token from Microsoft's OAuth flow
func getMicrosoftTokenFromWeb(config MicrosoftConfig) (*oauth2.Token, error) {
	// Create OAuth config
	msOAuthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/oauth2/v2.0/authorize", config.AuthorityURL),
			TokenURL: fmt.Sprintf("%s/oauth2/v2.0/token", config.AuthorityURL),
		},
		RedirectURL: config.RedirectURL,
		Scopes: []string{
			"https://graph.microsoft.com/.default",
			"offline_access",
		},
	}
	
	// Start local server for OAuth flow (similar to getTokenFromWeb for Google)
	listener, err := findAvailablePort([]int{8080, 8081, 8090})
	if err != nil {
		return nil, fmt.Errorf("unable to start listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	msOAuthConfig.RedirectURL = fmt.Sprintf("http://localhost:%d", port)

	codeChan := make(chan string)

	var server *http.Server
	server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			codeChan <- code
			fmt.Fprintf(w, "Authorization successful! You can close this window.")
			go func() {
				time.Sleep(time.Second)
				server.Shutdown(context.Background())
			}()
		}),
	}

	go server.Serve(listener)

	authURL := msOAuthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Please visit this URL to authorize Microsoft Graph access: \n%v\n", authURL)

	// Copy URL to clipboard
	err = copyUrlToClipboard(authURL)
	if err != nil {
		fmt.Printf("Failed to copy URL to clipboard: %v\n", err)
		fmt.Println("Please copy the URL manually and open it in your browser.")
	}

	code := <-codeChan

	tok, err := msOAuthConfig.Exchange(context.TODO(), code)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token: %v", err)
	}
	return tok, nil
}

// saveMicrosoftToken saves a token to the database
func saveMicrosoftToken(db *sql.DB, accountName string, token *oauth2.Token) error {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS microsoft_tokens (account_name TEXT PRIMARY KEY, token BLOB)")
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT OR REPLACE INTO microsoft_tokens (account_name, token) VALUES (?, ?)", accountName, tokenJSON)
	return err
}

// GetCalendar checks if the calendar exists and is accessible
func (m *MicrosoftCalendarProvider) GetCalendar(calendarID string) error {
	if m.useOAuth {
		// Using Microsoft Graph API
		_, err := m.graphClient.Users().ByUserId("me").Calendars().ByCalendarId(calendarID).Get(m.ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to get calendar: %w", err)
		}
	} else {
		// Using Exchange ActiveSync
		calendar, err := m.client.GetCalendar(m.ctx, calendarID)
		if err != nil {
			return fmt.Errorf("failed to get calendar: %w", err)
		}
		
		if calendar == nil {
			return fmt.Errorf("calendar not found: %s", calendarID)
		}
	}
	
	return nil
}

// AddEvent adds a new event to the calendar
func (m *MicrosoftCalendarProvider) AddEvent(calendarID string, event *Event) (string, error) {
	if m.useOAuth {
		// Using Microsoft Graph API
		newEvent := models.NewEvent()
		newEvent.SetSubject(&event.Summary)
		newEvent.SetBodyContent(&event.Description, models.BODYTYPE_TEXT)
		
		start := models.NewDateTimeTimeZone()
		start.SetDateTime(event.Start.Format("2006-01-02T15:04:05"))
		start.SetTimeZone("UTC")
		newEvent.SetStart(start)
		
		end := models.NewDateTimeTimeZone()
		end.SetDateTime(event.End.Format("2006-01-02T15:04:05"))
		end.SetTimeZone("UTC")
		newEvent.SetEnd(end)
		
		result, err := m.graphClient.Users().ByUserId("me").Calendars().ByCalendarId(calendarID).Events().Post(m.ctx, newEvent, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create event: %w", err)
		}
		
		return *result.GetId(), nil
	} else {
		// Using Exchange ActiveSync
		msEvent := &microsoft.CalendarEvent{
			Subject:     event.Summary,
			Body:        event.Description,
			StartTime:   event.Start,
			EndTime:     event.End,
			IsAllDay:    false,
			Sensitivity: "Normal",
			Status:      "Busy",
		}
		
		eventID, err := m.client.CreateEvent(m.ctx, calendarID, msEvent)
		if err != nil {
			return "", fmt.Errorf("failed to create event: %w", err)
		}
		
		return eventID, nil
	}
}

// UpdateEvent updates an existing event
func (m *MicrosoftCalendarProvider) UpdateEvent(calendarID string, eventID string, event *Event) error {
	if m.useOAuth {
		// Using Microsoft Graph API
		updatedEvent := models.NewEvent()
		updatedEvent.SetSubject(&event.Summary)
		updatedEvent.SetBodyContent(&event.Description, models.BODYTYPE_TEXT)
		
		start := models.NewDateTimeTimeZone()
		start.SetDateTime(event.Start.Format("2006-01-02T15:04:05"))
		start.SetTimeZone("UTC")
		updatedEvent.SetStart(start)
		
		end := models.NewDateTimeTimeZone()
		end.SetDateTime(event.End.Format("2006-01-02T15:04:05"))
		end.SetTimeZone("UTC")
		updatedEvent.SetEnd(end)
		
		_, err := m.graphClient.Users().ByUserId("me").Calendars().ByCalendarId(calendarID).Events().ByEventId(eventID).Patch(m.ctx, updatedEvent, nil)
		if err != nil {
			return fmt.Errorf("failed to update event: %w", err)
		}
	} else {
		// Using Exchange ActiveSync
		msEvent := &microsoft.CalendarEvent{
			ID:          eventID,
			Subject:     event.Summary,
			Body:        event.Description,
			StartTime:   event.Start,
			EndTime:     event.End,
			IsAllDay:    false,
			Sensitivity: "Normal",
			Status:      event.Status,
		}
		
		err := m.client.UpdateEvent(m.ctx, calendarID, eventID, msEvent)
		if err != nil {
			return fmt.Errorf("failed to update event: %w", err)
		}
	}
	
	return nil
}

// DeleteEvent deletes an event
func (m *MicrosoftCalendarProvider) DeleteEvent(calendarID string, eventID string) error {
	if m.useOAuth {
		// Using Microsoft Graph API
		err := m.graphClient.Users().ByUserId("me").Calendars().ByCalendarId(calendarID).Events().ByEventId(eventID).Delete(m.ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to delete event: %w", err)
		}
	} else {
		// Using Exchange ActiveSync
		err := m.client.DeleteEvent(m.ctx, calendarID, eventID)
		if err != nil {
			return fmt.Errorf("failed to delete event: %w", err)
		}
	}
	
	return nil
}

// ListEvents lists events in a calendar within a time range
func (m *MicrosoftCalendarProvider) ListEvents(calendarID string, timeMin, timeMax time.Time) ([]*Event, error) {
	if m.useOAuth {
		// Using Microsoft Graph API
		requestQuery := users.EventsRequestBuilderGetQueryParameters{
			StartDateTime: timeMin.Format(time.RFC3339),
			EndDateTime:   timeMax.Format(time.RFC3339),
		}
		configuration := users.EventsRequestBuilderGetRequestConfiguration{
			QueryParameters: &requestQuery,
		}
		
		result, err := m.graphClient.Users().ByUserId("me").Calendars().ByCalendarId(calendarID).Events().Get(m.ctx, &configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to list events: %w", err)
		}
		
		var events []*Event
		eventsPage := result.GetValue()
		for _, msEvent := range eventsPage {
			start, _ := time.Parse("2006-01-02T15:04:05Z", *msEvent.GetStart().GetDateTime())
			end, _ := time.Parse("2006-01-02T15:04:05Z", *msEvent.GetEnd().GetDateTime())
			subject := *msEvent.GetSubject()
			id := *msEvent.GetId()
			
			var description string
			if msEvent.GetBody() != nil && msEvent.GetBody().GetContent() != nil {
				description = *msEvent.GetBody().GetContent()
			}
			
			events = append(events, &Event{
				ID:          id,
				Summary:     subject,
				Description: description,
				Start:       start,
				End:         end,
				Status:      "confirmed", // Default status
			})
		}
		
		return events, nil
	} else {
		// Using Exchange ActiveSync
		msEvents, err := m.client.GetEvents(m.ctx, calendarID, timeMin, timeMax)
		if err != nil {
			return nil, fmt.Errorf("failed to list events: %w", err)
		}
		
		var events []*Event
		for _, msEvent := range msEvents {
			event := &Event{
				ID:          msEvent.ID,
				Summary:     msEvent.Subject,
				Description: msEvent.Body,
				Start:       msEvent.StartTime,
				End:         msEvent.EndTime,
				Status:      msEvent.Status,
			}
			events = append(events, event)
		}
		
		return events, nil
	}
}

// GetEvent gets a specific event
func (m *MicrosoftCalendarProvider) GetEvent(calendarID string, eventID string) (*Event, error) {
	if m.useOAuth {
		// Using Microsoft Graph API
		msEvent, err := m.graphClient.Users().ByUserId("me").Calendars().ByCalendarId(calendarID).Events().ByEventId(eventID).Get(m.ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get event: %w", err)
		}
		
		start, _ := time.Parse("2006-01-02T15:04:05Z", *msEvent.GetStart().GetDateTime())
		end, _ := time.Parse("2006-01-02T15:04:05Z", *msEvent.GetEnd().GetDateTime())
		subject := *msEvent.GetSubject()
		id := *msEvent.GetId()
		
		var description string
		if msEvent.GetBody() != nil && msEvent.GetBody().GetContent() != nil {
			description = *msEvent.GetBody().GetContent()
		}
		
		return &Event{
			ID:          id,
			Summary:     subject,
			Description: description,
			Start:       start,
			End:         end,
			Status:      "confirmed", // Default status
		}, nil
	} else {
		// Using Exchange ActiveSync
		msEvent, err := m.client.GetEvent(m.ctx, calendarID, eventID)
		if err != nil {
			return nil, fmt.Errorf("failed to get event: %w", err)
		}
		
		event := &Event{
			ID:          msEvent.ID,
			Summary:     msEvent.Subject,
			Description: msEvent.Body,
			Start:       msEvent.StartTime,
			End:         msEvent.EndTime,
			Status:      msEvent.Status,
		}
		
		return event, nil
	}
}