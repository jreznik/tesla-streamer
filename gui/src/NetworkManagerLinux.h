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

#ifndef NETWORKMANAGERLINUX_H
#define NETWORKMANAGERLINUX_H

#include "INetworkManager.h"
#include <QtDBus/QDBusInterface>

class NetworkManagerLinux : public INetworkManager {
    Q_OBJECT

public:
    NetworkManagerLinux(QObject *parent = nullptr);
    ~NetworkManagerLinux();

    bool startHotspot(const QString &ssid, const QString &password) override;
    void stopHotspot() override;
    bool isHotspotActive() const override;
    QString getHotspotUrl() const override;
    QString getStatusMessage() const override;

private:
    bool m_active;
    QString m_status;
    QString m_activeConnectionPath;
};

#endif
