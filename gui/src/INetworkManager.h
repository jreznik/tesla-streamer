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

#ifndef INETWORKMANAGER_H
#define INETWORKMANAGER_H

#include <QString>
#include <QObject>

class INetworkManager : public QObject {
    Q_OBJECT

public:
    virtual ~INetworkManager() {}

    virtual bool startHotspot(const QString &ssid, const QString &password, const QString &interface = "") = 0;
    virtual void stopHotspot() = 0;
    virtual bool isHotspotActive() const = 0;
    virtual QString getHotspotUrl() const = 0;
    virtual QString getStatusMessage() const = 0;
    virtual QStringList getAvailableInterfaces() const = 0;

signals:
    void hotspotStateChanged(bool active);
    void errorOccurred(const QString &error);
    void messageLogged(const QString &msg);
};

#endif
