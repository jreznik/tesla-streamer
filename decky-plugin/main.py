# Tesla Streamer - High-performance screen streaming for Tesla browsers
# Copyright (C) 2026 Jaroslav Reznik
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

import os
import subprocess
import logging
import json
import asyncio
import dbus
import requests

logging.basicConfig(level=logging.INFO)

FLATPAK_ID = "io.github.jreznik.TeslaStreamer"

class Plugin:
    def __init__(self):
        self.process = None
        self.hotspot_active = False

    async def is_installed(self):
        try:
            # Check if flatpak is installed
            res = subprocess.run(["flatpak", "info", FLATPAK_ID], capture_output=True, text=True)
            return res.returncode == 0
        except Exception as e:
            logging.error(f"Error checking flatpak installation: {e}")
            return False

    async def start_server(self, profile="latency"):
        if self.process:
            return {"success": False, "error": "Server already running"}
        
        try:
            cmd = ["flatpak", "run", FLATPAK_ID, "--profile", profile]
            # Use subprocess.Popen for long running process
            self.process = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            return {"success": True}
        except Exception as e:
            logging.error(f"Error starting server: {e}")
            return {"success": False, "error": str(e)}

    async def stop_server(self):
        if not self.process:
            return {"success": True}
        
        try:
            self.process.terminate()
            self.process = None
            return {"success": True}
        except Exception as e:
            logging.error(f"Error stopping server: {e}")
            return {"success": False, "error": str(e)}

    async def update_config(self, config):
        try:
            res = requests.post("http://localhost:8080/api/config", json=config, timeout=2)
            return {"success": res.status_code == 200}
        except Exception as e:
            logging.error(f"Error updating config: {e}")
            return {"success": False, "error": str(e)}

    async def reselect_source(self):
        try:
            res = requests.get("http://localhost:8080/api/reselect", timeout=2)
            return {"success": res.status_code == 200}
        except Exception as e:
            logging.error(f"Error requesting reselection: {e}")
            return {"success": False, "error": str(e)}

    async def start_hotspot(self):
        try:
            bus = dbus.SystemBus()
            nm = bus.get_object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
            # Replicating C++ logic via DBus... 
            # (Detailed DBus logic will be expanded here)
            self.hotspot_active = True
            return {"success": True, "message": "Hotspot started (Native logic to be completed)"}
        except Exception as e:
            logging.error(f"Hotspot error: {e}")
            return {"success": False, "error": str(e)}

    async def stop_hotspot(self):
        self.hotspot_active = False
        return {"success": True}

    async def get_status(self):
        installed = await self.is_installed()
        return {
            "installed": installed,
            "server_running": self.process is not None and self.process.poll() is None,
            "hotspot_active": self.hotspot_active
        }

    async def _main(self):
        logging.info("Tesla Streamer plugin started")

    async def _unload(self):
        await self.stop_server()
        logging.info("Tesla Streamer plugin unloaded")
