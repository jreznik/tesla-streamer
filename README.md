# Tesla Streamer

High-performance, ultra-low latency screen streaming for Tesla browsers.

## Overview

Tesla Streamer allows you to stream your laptop or SteamDeck screen directly to your Tesla car's web browser. It is specifically designed to bypass common networking restrictions and provide a smooth, responsive experience suitable for gaming and general desktop use.

### Features
- **Ultra-Low Latency:** Uses WebRTC and H.264 `zerolatency` tuning.
- **Native Capture:** Leverages Wayland and PipeWire (via XDG Desktop Portal) for high-performance capture on modern Linux (including SteamDeck Game Mode).
- **One-Click Tesla Mode:** Automatic Wi-Fi hotspot and network configuration to bypass Tesla browser restrictions.
- **Dynamic Quality Control:** Adjust profiles (Latency, Balanced, Quality) and display modes in real-time from the GUI.
- **System Tray Integration:** Manage your stream and reselect sources from a convenient tray icon.
- **Professional Dashboard:** A native C++ Qt6 GUI for easy management.

## Installation

### Prerequisites (Fedora)
```bash
sudo dnf install gstreamer1-devel gstreamer1-plugins-base-devel \
                 qt6-qtbase-devel libayatana-appindicator-gtk3-devel \
                 cmake gcc-c++ golang
```

### Build
Clone the repository and run the build script:
```bash
./build.sh
```

### Flatpak
You can also build and run as a Flatpak:
```bash
flatpak-builder --force-clean build-dir io.github.jreznik.TeslaStreamer.yaml
flatpak-builder --run build-dir io.github.jreznik.TeslaStreamer.yaml tesla-streamer-gui
```

### Decky Loader Plugin (Steam Deck Only)
A native Decky Loader plugin is available in the `decky-plugin/` directory.
1. Ensure the **Tesla Streamer Flatpak** is installed on your Steam Deck.
2. Build the plugin:
   ```bash
   cd decky-plugin
   pnpm install
   pnpm run build
   ```
3. Deploy the plugin to your Deck via `decky-cli` or by copying the `dist` folder to `~/homebrew/plugins/TeslaStreamer`.
4. The plugin will appear in the Steam Deck's Quick Access Menu (QAM), providing native controls for the hotspot and streaming engine.

## Usage
1. Launch the application: `./gui/build/tesla-streamer-gui`.
2. Click **Start Tesla Mode** to initialize the Wi-Fi hotspot (if needed).
3. Connect your Tesla to the Wi-Fi network (SSID: `TeslaStreamer`).
4. Click **Start Server** and select the window or screen you want to share.
5. In your Tesla browser, navigate to the URL shown in the GUI (e.g., `http://play.tesla.stream:8080`).
6. Click **LAUNCH STREAM** on the web page.

## Firewall Configuration (firewalld)
If you are using `firewalld` (default on Fedora), you need to open the following ports to allow the Tesla to connect:

```bash
# Open signaling and web server port
sudo firewall-cmd --add-port=8080/tcp --permanent
# Open DNS port for offline spoofing
sudo firewall-cmd --add-port=53/udp --permanent
# Open WebRTC media ports
sudo firewall-cmd --add-port=49152-65535/udp --permanent
# Reload firewall
sudo firewall-cmd --reload
```

## Credits & Author

- **Author:** Jaroslav Reznik
- **AI Assistance:** Developed with significant assistance from **Google Gemini**, which helped design the architecture, implement the Go/C++ components, and optimize the streaming pipeline.

## License

This project is licensed under the **GNU General Public License v3 (GPLv3)**. See the [LICENSE](LICENSE) file for the full text.

---
Copyright (C) 2026 Jaroslav Reznik
