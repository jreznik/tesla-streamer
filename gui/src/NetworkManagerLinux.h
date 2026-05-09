#ifndef NETWORKMANAGERLINUX_H
#define NETWORKMANAGERLINUX_H

#include "INetworkManager.h"
#include <QtDBus/QDBusInterface>
#include <QMap>

class NetworkManagerLinux : public INetworkManager {
    Q_OBJECT

public:
    NetworkManagerLinux(QObject *parent = nullptr);
    ~NetworkManagerLinux();

    bool startHotspot(const QString &ssid, const QString &password, const QString &interface = "") override;
    void stopHotspot() override;
    bool isHotspotActive() const override;
    QString getHotspotUrl() const override;
    QString getStatusMessage() const override;
    QStringList getAvailableInterfaces() const override;

private:
    void refreshInterfaces();

    bool m_active;
    QString m_status;
    QString m_connectionPath;
    QString m_activeConnectionPath;
    QMap<QString, QString> m_interfaces; // Name -> Path
};

#endif
