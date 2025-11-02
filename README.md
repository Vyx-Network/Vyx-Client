# Vyx Desktop Client

Desktop client for the Vyx bandwidth sharing network. Share your unused bandwidth and earn credits.

## Features

- 🌐 **Cross-platform support** - Windows, macOS, and Linux
- 🔒 **Secure QUIC protocol** - Fast and encrypted connections
- 🎯 **System tray integration** - Easy access and control
- 🔄 **Automatic reconnection** - Reliable bandwidth sharing
- 🌍 **Smart server selection** - Automatically connects to optimal server
- 🚀 **Easy authentication** - Simple browser-based login
- ⚙️ **Auto-start on boot** - Configurable via tray menu

## Installation

### Download Pre-built Binary

Download the latest release from the [Releases](https://github.com/Vyx-Network/Vyx-Client/releases) page.

### Build from Source

**Prerequisites:**
- Go 1.25 or higher

**Build commands:**

```bash
# Clone the repository
git clone https://github.com/Vyx-Network/Vyx-Client.git
cd vyx-client

# Build console version (with terminal output)
go build -o vyx-client

# Build GUI version (Windows - no console window)
go build -ldflags="-H windowsgui" -o vyx-client.exe

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o vyx-client-linux
GOOS=darwin GOARCH=amd64 go build -o vyx-client-macos
GOOS=windows GOARCH=amd64 go build -o vyx-client.exe
```

**Quick build scripts:**
- Windows: `build-console.bat` or `build-gui.bat`
- Linux/macOS: `chmod +x build.sh && ./build.sh`

## Usage

1. **Launch the client**
   - Windows: Double-click `vyx-client.exe`
   - macOS/Linux: Run `./vyx-client`

2. **First time setup**
   - The browser will automatically open for authentication
   - Login or create an account
   - Return to the desktop app

3. **Start sharing**
   - Click the Vyx icon in your system tray
   - Click "Start Sharing" to begin earning
   - Monitor your status and connections in real-time

4. **Control sharing**
   - **Stop Sharing** - Pause bandwidth sharing
   - **Dashboard** - View earnings and statistics
   - **Run at Startup** - Toggle auto-start on boot
   - **Logout** - Sign out and stop sharing

## System Tray Menu

```
┌─────────────────────────────────┐
│ Vyx - Proxy Node Client         │
├─────────────────────────────────┤
│ Status: Connected               │
│ Uptime: 2h 34m 12s             │
│ Active Connections: 5          │
├─────────────────────────────────┤
│ Start Sharing                  │ (or Stop Sharing when active)
│ Dashboard                      │
│ Logout                         │
├─────────────────────────────────┤
│ ☑ Run at Startup               │
├─────────────────────────────────┤
│ Quit                            │
└─────────────────────────────────┘
```

## Configuration

Configuration is automatically stored in:

- **Windows:** `%APPDATA%\Vyx\config.json`
- **macOS:** `~/Library/Application Support/Vyx/config.json`
- **Linux:** `~/.config/vyx/config.json`

### Configuration File Structure

```json
{
  "server_url": "api.vyx.network:8443",
  "user_id": "your-user-id",
  "email": "your@email.com",
  "verbose_logging": false,
  "auto_start": true
}
```

**Note:** API tokens are stored securely in your system's credential manager (not in the config file).

## Logging

Logs are automatically saved to:

- **Windows:** `%APPDATA%\Vyx\logs\vyx-YYYY-MM-DD.log`
- **macOS:** `~/Library/Logs/Vyx/vyx-YYYY-MM-DD.log`
- **Linux:** `~/.vyx/logs/vyx-YYYY-MM-DD.log`

## Troubleshooting

### Connection Issues

If you're experiencing connection problems:

1. Check your internet connection
2. Verify firewall isn't blocking the app
3. Try the "Stop Sharing" → "Start Sharing" cycle
4. Check logs for error messages

### Authentication Problems

If authentication fails:

1. Click "Logout" in the tray menu
2. Click "Login" to re-authenticate
3. Ensure cookies are enabled in your browser
4. Try a different browser if issues persist

### Performance Issues

- Disable verbose logging for better performance
- Check system resources (CPU/Memory)
- Ensure no other proxy/VPN software conflicts

## Development

### Project Structure

```
vyx-client/
├── assets/          # Icons and resources
├── auth/            # Authentication logic
├── config/          # Configuration management
├── conn/            # Connection and QUIC protocol
├── logger/          # Logging utilities
├── platform/        # Platform-specific code (autostart)
├── ui/              # System tray UI
├── main.go          # Entry point
└── go.mod           # Go dependencies
```

### Dependencies

- [quic-go](https://github.com/quic-go/quic-go) - QUIC protocol implementation
- [systray](https://github.com/getlantern/systray) - System tray integration
- [keyring](https://github.com/zalando/go-keyring) - Secure credential storage

### Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## Security

- Credentials are stored in your system's secure credential manager (Windows Credential Manager, macOS Keychain, Linux Secret Service)
- All connections use encrypted QUIC protocol
- API tokens are never logged or exposed
- See [SECURITY.md](SECURITY.md) for reporting vulnerabilities

## License

[MIT License](LICENSE) - see LICENSE file for details

## Links

- [Website](https://vyx.network)
- [Dashboard](https://vyx.network/dashboard)
- [Documentation](https://docs.vyx.network)
- [Support](https://vyx.network/support)

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history and release notes.

---

**Made with ❤️ by the Vyx Team**
