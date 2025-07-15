# Evilginx 3.0 with Web Dashboard

A modern web dashboard for Evilginx 3.0 that provides real-time monitoring of phishing sessions, credentials, and tokens without requiring terminal access.

## 🚀 Features

### Web Dashboard
- **Real-time monitoring** with WebSocket updates
- **Modern dark theme** responsive UI
- **Session management** with detailed views and export functionality
- **Statistics overview** (total/active/completed sessions, credentials, tokens)
- **Activity feed** with real-time events
- **Token export** as JSON files
- **Mobile responsive** design

### Enhanced Evilginx Integration
- Seamless integration with existing Evilginx 3.0 features
- Real-time notifications for new sessions and credential captures
- Export functionality for captured data
- Performance optimizations for heavy traffic

## 📋 Quick Start

### Ubuntu/Linux (Recommended)
```bash
# Download and run the automated setup script
curl -O https://raw.githubusercontent.com/YOUR_USERNAME/evilginx3-dashboard/main/ubuntu_setup.sh
chmod +x ubuntu_setup.sh
sudo ./ubuntu_setup.sh
```

### Manual Installation
```bash
# Clone the repository
git clone https://github.com/YOUR_USERNAME/evilginx3-dashboard.git
cd evilginx3-dashboard

# Install dependencies
go mod download

# Build the project
go build -o evilginx2 main.go

# Start with dashboard
./evilginx2 -dashboard
```

## 🖥️ Dashboard Access

Once started with the `-dashboard` flag:
- **Dashboard URL**: http://localhost:8080
- **Default credentials**: No authentication required (add authentication for production)

## 📊 Dashboard Features

### Session Monitoring
- View all active and completed phishing sessions
- Real-time session status updates
- Detailed session information including IP, User-Agent, and timestamps
- Export session data as JSON

### Statistics Overview
- Total sessions count
- Active sessions monitoring
- Credentials captured counter
- Tokens captured counter
- Success rate calculations

### Activity Feed
- Real-time event notifications
- Session creation alerts
- Credential capture notifications
- Token capture alerts

## 🔧 Configuration

### Command Line Options
```bash
# Start with dashboard
./evilginx2 -dashboard

# Custom dashboard port
./evilginx2 -dashboard -port 9090

# Enable debug mode
./evilginx2 -dashboard -debug
```

### Performance Tuning
For heavy traffic scenarios, apply the performance patches:
```bash
# Apply performance optimizations
cp dashboard_performance_patch.go core/dashboard.go
go build -o evilginx2 main.go
```

## 📚 Documentation

- **[Dashboard README](DASHBOARD_README.md)** - Detailed dashboard documentation
- **[Ubuntu Compatibility](UBUNTU_COMPATIBILITY.md)** - Ubuntu deployment guide
- **[Deployment Summary](DEPLOYMENT_SUMMARY.md)** - Quick deployment reference
- **[Performance Analysis](PERFORMANCE_ANALYSIS.md)** - Performance optimization guide
- **[Heavy Traffic Analysis](HEAVY_TRAFFIC_ANALYSIS.md)** - High-load deployment guide

## 🚨 Performance Considerations

### Current Implementation
- Handles 10-20 concurrent users
- Response time: 5-10 seconds with 1000+ sessions
- May freeze under heavy load

### With Performance Patches
- Handles 100+ concurrent connections
- Response time: <500ms
- No system freezes under load
- Stable memory usage

## 🛡️ Security Notes

⚠️ **Important**: This tool is for educational and authorized testing purposes only. Ensure you have proper authorization before using this tool.

### Production Deployment
- Add authentication to the dashboard
- Use HTTPS in production
- Implement rate limiting
- Monitor resource usage
- Regular security updates

## 🔄 API Endpoints

The dashboard provides REST API endpoints:
- `GET /api/sessions` - Get all sessions
- `GET /api/stats` - Get statistics
- `GET /api/activity` - Get activity feed
- `POST /api/sessions/{id}/export` - Export session data
- `WebSocket /ws` - Real-time updates

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Original Evilginx by [@kgretzky](https://github.com/kgretzky)
- Dashboard implementation and performance optimizations
- Community contributions and feedback

## 📞 Support

- Create an issue for bug reports
- Check existing documentation
- Review performance guides for optimization

---

**Disclaimer**: This tool is intended for educational and authorized security testing purposes only. Users are responsible for complying with applicable laws and regulations.
