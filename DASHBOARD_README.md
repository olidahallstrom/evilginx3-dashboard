# Evilginx2 Web Dashboard

This enhanced version of Evilginx2 includes a **real-time web dashboard** that allows you to monitor incoming phishing data without accessing the terminal.

## Features

### 🌐 Web Dashboard
- **Real-time monitoring** of sessions, credentials, and tokens
- **Live WebSocket updates** - no need to refresh the page
- **Modern responsive UI** with dark theme
- **Session management** with detailed views
- **Token export** functionality
- **Statistics overview** with key metrics

### 📊 Dashboard Components
1. **Statistics Cards**: Total sessions, active sessions, completed sessions, credentials, tokens, and today's activity
2. **Recent Sessions Panel**: Live view of incoming sessions with status indicators
3. **Activity Feed**: Real-time activity log with different severity levels
4. **Session Details Modal**: Detailed view of captured tokens and credentials
5. **Export Functionality**: Download session data as JSON files

## Usage

### Starting with Dashboard
```bash
# Start with dashboard on default port (8080)
./evilginx2 -dashboard 8080

# Or use the convenience script
./start_with_dashboard.sh

# Start with dashboard on custom port
./evilginx2 -dashboard 9090

# Disable dashboard (traditional terminal-only mode)
./evilginx2 -dashboard 0
```

### Accessing the Dashboard
1. Start Evilginx2 with the dashboard enabled
2. Open your browser and navigate to `http://localhost:8080` (or your custom port)
3. The dashboard will automatically connect via WebSocket for real-time updates

### Dashboard Interface

#### Statistics Overview
- **Total Sessions**: All sessions created
- **Active Sessions**: Currently active sessions
- **Completed**: Sessions with captured credentials
- **Credentials**: Total credentials captured
- **Tokens**: Total authentication tokens captured
- **Today**: Sessions created today

#### Session Management
- **Session List**: Shows recent sessions with status indicators
- **Session Details**: Click "View Details" to see:
  - Session information (ID, phishlet, IP, user agent, etc.)
  - Cookie tokens (formatted JSON)
  - Body tokens (from POST requests)
  - HTTP header tokens
  - Export functionality

#### Real-time Updates
- **New sessions** appear automatically
- **Credential captures** are highlighted
- **Token captures** are logged in activity feed
- **Connection status** indicator shows WebSocket status

## Configuration

### Telegram Integration
The dashboard works alongside the existing Telegram notifications:
1. Configure Telegram in `~/.evilginx/config.json`:
```json
{
  "general": {
    "chatid": "your_chat_id",
    "teletoken": "your_bot_token"
  }
}
```

### Security Considerations
- The dashboard runs on localhost by default
- No authentication is implemented (intended for local use)
- For production use, consider adding authentication
- Use HTTPS in production environments

## Technical Details

### Architecture
- **Backend**: Go HTTP server with WebSocket support
- **Frontend**: Vanilla JavaScript with WebSocket client
- **Database**: Same BuntDB as original Evilginx2
- **Real-time**: WebSocket-based live updates

### API Endpoints
- `GET /` - Dashboard interface
- `GET /api/sessions` - List sessions
- `GET /api/sessions/{id}` - Get session details
- `GET /api/sessions/{id}/tokens` - Get session tokens
- `GET /api/sessions/{id}/export` - Export session data
- `GET /api/stats` - Get statistics
- `GET /api/phishlets` - List phishlets
- `GET /api/activity` - Get recent activity
- `WebSocket /ws` - Real-time updates

### WebSocket Messages
- `new_session` - New session created
- `session_update` - Session updated
- `credential_captured` - Credentials captured
- `stats_update` - Statistics updated
- `activity` - Activity event

## Customization

### Styling
The dashboard uses a dark theme with the following color scheme:
- **Background**: `#1a1a1a`
- **Cards**: `#2d2d2d`
- **Accent**: `#4ecdc4` (teal)
- **Error**: `#ff6b6b` (red)
- **Success**: `#66bb6a` (green)

### Adding Features
To extend the dashboard:
1. Add new API endpoints in `core/dashboard.go`
2. Update the frontend JavaScript
3. Add new WebSocket message types
4. Implement real-time notifications

## Troubleshooting

### Common Issues
1. **Dashboard not loading**: Check if the port is available
2. **WebSocket connection failed**: Verify firewall settings
3. **No real-time updates**: Check browser console for errors
4. **Sessions not appearing**: Ensure database permissions

### Debugging
- Use browser developer tools to inspect WebSocket messages
- Check terminal output for error messages
- Verify configuration in `~/.evilginx/config.json`

## Integration with Existing Features

### Phishlets
- All existing phishlets work unchanged
- Dashboard shows phishlet information
- Session data includes phishlet details

### Lures
- Lure creation through terminal interface
- Dashboard shows lure-generated sessions
- Real-time lure activity monitoring

### Certificates
- Existing certificate management unchanged
- Dashboard works with both development and production certificates

## Performance

### Optimization
- WebSocket connections are managed efficiently
- Session data is cached for quick retrieval
- Database queries are optimized
- Frontend updates are throttled to prevent overload

### Scalability
- Dashboard handles hundreds of concurrent sessions
- WebSocket connections are lightweight
- Memory usage is optimized for long-running sessions

## Future Enhancements

### Planned Features
- **Authentication system** for multi-user environments
- **Advanced filtering** and search capabilities
- **Historical data visualization** with charts
- **Mobile-responsive design** improvements
- **Plugin system** for custom extensions
- **API rate limiting** and security features
- **Export formats** (CSV, XML, etc.)
- **Automated reporting** capabilities

### Contributing
To contribute to the dashboard:
1. Fork the repository
2. Create a feature branch
3. Implement your changes
4. Test thoroughly
5. Submit a pull request

## License
This enhancement maintains the same license as the original Evilginx2 project. 