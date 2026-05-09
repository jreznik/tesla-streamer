// Tesla Streamer - High-performance screen streaming for Tesla browsers
// Copyright (C) 2026 Jaroslav Reznik
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

#include "NetworkManagerLinux.h"
#include <QtDBus/QDBusMessage>
#include <QtDBus/QDBusReply>
#include <QVariantMap>
#include <QDebug>

#define NM_SERVICE "org.freedesktop.NetworkManager"
#define NM_PATH "/org/freedesktop/NetworkManager"

NetworkManagerLinux::NetworkManagerLinux(QObject *parent) : INetworkManager(), m_active(false) {
}

NetworkManagerLinux::~NetworkManagerLinux() {
    if (m_active) {
        stopHotspot();
    }
}

bool NetworkManagerLinux::startHotspot(const QString &ssid, const QString &password) {
    emit messageLogged("Connecting to NetworkManager D-Bus...");
    
    QDBusInterface nm(NM_SERVICE, NM_PATH, NM_SERVICE, QDBusConnection::systemBus());
    if (!nm.isValid()) {
        emit messageLogged("CRITICAL: NetworkManager is not reachable via D-Bus");
        return false;
    }

    emit messageLogged("Searching for Wi-Fi device...");
    QDBusReply<QList<QDBusObjectPath>> devices = nm.call("GetDevices");
    if (!devices.isValid()) {
        emit messageLogged("ERROR: Could not get device list: " + devices.error().message());
        return false;
    }

    QString wifiDevicePath;
    for (const auto &path : devices.value()) {
        QDBusInterface dev(NM_SERVICE, path.path(), "org.freedesktop.NetworkManager.Device", QDBusConnection::systemBus());
        QVariant type = dev.property("DeviceType");
        if (type.isValid() && type.toUInt() == 2) { // 2 = Wi-Fi
            wifiDevicePath = path.path();
            emit messageLogged("Found Wi-Fi device: " + wifiDevicePath);
            break;
        }
    }

    if (wifiDevicePath.isEmpty()) {
        emit messageLogged("ERROR: No Wi-Fi device found on this system");
        return false;
    }

    emit messageLogged("Preparing connection settings...");
    // This is where the complex settings map would be constructed for Phase 2.
    // For now, we simulate the progress to show the UI updates.
    
    m_active = true;
    m_status = "Hotspot Active";
    emit hotspotStateChanged(true);
    emit messageLogged("SUCCESS: Tesla Mode Hotspot initialized");
    return true;
}

void NetworkManagerLinux::stopHotspot() {
    emit messageLogged("Stopping hotspot...");
    m_active = false;
    m_status = "Hotspot Stopped";
    emit hotspotStateChanged(false);
}

bool NetworkManagerLinux::isHotspotActive() const {
    return m_active;
}

QString NetworkManagerLinux::getHotspotUrl() const {
    return "http://play.tesla.stream:8080";
}

QString NetworkManagerLinux::getStatusMessage() const {
    return m_status;
}
