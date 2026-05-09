#include "NetworkManagerLinux.h"
#include <QtDBus/QDBusMessage>
#include <QtDBus/QDBusReply>
#include <QtDBus/QDBusObjectPath>
#include <QtDBus/QDBusMetaType>
#include <QVariantMap>
#include <QUuid>
#include <QDebug>

#define NM_SERVICE "org.freedesktop.NetworkManager"
#define NM_PATH "/org/freedesktop/NetworkManager"

NetworkManagerLinux::NetworkManagerLinux(QObject *parent) : INetworkManager(), m_active(false) {
    qDBusRegisterMetaType<QMap<QString, QVariantMap>>();
    refreshInterfaces();
}

NetworkManagerLinux::~NetworkManagerLinux() {
    if (m_active) {
        stopHotspot();
    }
}

void NetworkManagerLinux::refreshInterfaces() {
    m_interfaces.clear();
    QDBusInterface nm(NM_SERVICE, NM_PATH, NM_SERVICE, QDBusConnection::systemBus());
    QDBusReply<QList<QDBusObjectPath>> devices = nm.call("GetDevices");
    
    if (devices.isValid()) {
        for (const auto &path : devices.value()) {
            QDBusInterface dev(NM_SERVICE, path.path(), "org.freedesktop.NetworkManager.Device", QDBusConnection::systemBus());
            if (dev.property("DeviceType").toUInt() == 2) { // Wi-Fi
                QString name = dev.property("Interface").toString();
                m_interfaces[name] = path.path();
            }
        }
    }
}

QStringList NetworkManagerLinux::getAvailableInterfaces() const {
    const_cast<NetworkManagerLinux*>(this)->refreshInterfaces();
    return m_interfaces.keys();
}

bool NetworkManagerLinux::startHotspot(const QString &ssid, const QString &password, const QString &interface) {
    emit messageLogged("Cleaning up old TeslaStreamer profiles...");
    
    QDBusInterface settingsInterface(NM_SERVICE, "/org/freedesktop/NetworkManager/Settings", "org.freedesktop.NetworkManager.Settings", QDBusConnection::systemBus());
    QDBusReply<QList<QDBusObjectPath>> connections = settingsInterface.call("ListConnections");
    
    if (connections.isValid()) {
        for (const auto &connPath : connections.value()) {
            QDBusInterface conn(NM_SERVICE, connPath.path(), "org.freedesktop.NetworkManager.Settings.Connection", QDBusConnection::systemBus());
            QDBusMessage reply = conn.call("GetSettings");
            
            if (reply.type() != QDBusMessage::ErrorMessage) {
                const QDBusArgument arg = reply.arguments().at(0).value<QDBusArgument>();
                QMap<QString, QVariantMap> settingsMap;
                arg >> settingsMap;
                
                if (settingsMap.contains("connection") && settingsMap["connection"].value("id").toString() == ssid) {
                    emit messageLogged("Deleting stale profile: " + connPath.path());
                    conn.call("Delete");
                }
            }
        }
    }

    refreshInterfaces();
    
    QString wifiDevicePath;
    if (!interface.isEmpty() && m_interfaces.contains(interface)) {
        wifiDevicePath = m_interfaces[interface];
    } else if (!m_interfaces.isEmpty()) {
        wifiDevicePath = m_interfaces.first();
    }

    if (wifiDevicePath.isEmpty()) {
        emit messageLogged("ERROR: No Wi-Fi device found");
        return false;
    }

    QVariantMap connection;
    connection["id"] = ssid;
    connection["uuid"] = QUuid::createUuid().toString().remove('{').remove('}');
    connection["type"] = "802-11-wireless";
    connection["autoconnect"] = false;

    QVariantMap wireless;
    wireless["ssid"] = ssid.toUtf8();
    wireless["mode"] = "ap";
    wireless["band"] = "bg";
    wireless["security"] = "802-11-wireless-security";

    QVariantMap security;
    security["key-mgmt"] = "wpa-psk";
    security["psk"] = password;

    QVariantMap ipv4;
    ipv4["method"] = "shared";

    QVariantMap ipv6;
    ipv6["method"] = "ignore";

    QMap<QString, QVariantMap> settings;
    settings["connection"] = connection;
    settings["802-11-wireless"] = wireless;
    settings["802-11-wireless-security"] = security;
    settings["ipv4"] = ipv4;
    settings["ipv6"] = ipv6;

    emit messageLogged("Activating Hotspot on " + (interface.isEmpty() ? "default device" : interface) + "...");
    
    QDBusMessage msg = QDBusMessage::createMethodCall(NM_SERVICE, NM_PATH, NM_SERVICE, "AddAndActivateConnection");
    msg << QVariant::fromValue(settings) << QDBusObjectPath(wifiDevicePath) << QDBusObjectPath("/");
    
    QDBusMessage reply = QDBusConnection::systemBus().call(msg);
    if (reply.type() == QDBusMessage::ErrorMessage) {
        emit messageLogged("ERROR: Failed to activate hotspot: " + reply.errorMessage());
        return false;
    }

    m_connectionPath = reply.arguments().at(0).value<QDBusObjectPath>().path();
    m_activeConnectionPath = reply.arguments().at(1).value<QDBusObjectPath>().path();
    
    m_active = true;
    m_status = "Hotspot Active";
    emit hotspotStateChanged(true);
    emit messageLogged("SUCCESS: Hotspot visible. Diagnostics active.");
    return true;
}

void NetworkManagerLinux::stopHotspot() {
    if (!m_active) return;
    
    emit messageLogged("Deactivating hotspot...");
    QDBusInterface nm(NM_SERVICE, NM_PATH, NM_SERVICE, QDBusConnection::systemBus());
    nm.call("DeactivateConnection", QDBusObjectPath(m_activeConnectionPath));
    
    emit messageLogged("Removing temporary profile...");
    QDBusInterface conn(NM_SERVICE, m_connectionPath, "org.freedesktop.NetworkManager.Settings.Connection", QDBusConnection::systemBus());
    conn.call("Delete");
    
    m_active = false;
    m_status = "Hotspot Stopped";
    emit hotspotStateChanged(false);
}

bool NetworkManagerLinux::isHotspotActive() const {
    return m_active;
}

QString NetworkManagerLinux::getHotspotUrl() const {
    return "http://10.42.0.1:8080";
}

QString NetworkManagerLinux::getStatusMessage() const {
    return m_status;
}
