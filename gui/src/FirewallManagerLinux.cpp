#include "FirewallManagerLinux.h"
#include <QtDBus/QDBusMessage>
#include <QtDBus/QDBusReply>
#include <QDebug>

#define FIREWALLD_SERVICE "org.fedoraproject.FirewallD1"
#define FIREWALLD_PATH "/org/fedoraproject/FirewallD1"
#define FIREWALLD_IFACE_ZONE "org.fedoraproject.FirewallD1.zone"

FirewallManagerLinux::FirewallManagerLinux(QObject *parent) : IFirewallManager() {
}

FirewallManagerLinux::~FirewallManagerLinux() {
}

bool FirewallManagerLinux::configureFirewall() {
    emit messageLogged("Connecting to firewalld D-Bus...");
    
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_SERVICE, QDBusConnection::systemBus());
    if (!fw.isValid()) {
        emit messageLogged("ERROR: firewalld is not reachable. Is it running?");
        return false;
    }

    emit messageLogged("Retrieving active firewall zones...");
    // getActiveZones returns a{sas} (Map of string to array of strings)
    QDBusMessage msg = QDBusMessage::createMethodCall(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, "getActiveZones");
    QDBusMessage reply = QDBusConnection::systemBus().call(msg);
    
    QStringList zonesToConfigure;
    if (reply.type() != QDBusMessage::ErrorMessage) {
        // Correctly parse a{sas}
        const QDBusArgument arg = reply.arguments().at(0).value<QDBusArgument>();
        QMap<QString, QStringList> activeZones;
        arg >> activeZones;
        zonesToConfigure = activeZones.keys();
    }

    if (zonesToConfigure.isEmpty()) {
        emit messageLogged("No active zones found, falling back to default zone.");
        QDBusReply<QString> defaultZone = fw.call("getDefaultZone");
        if (defaultZone.isValid()) {
            zonesToConfigure << defaultZone.value();
        }
    }

    bool success = true;
    for (const QString &zoneName : zonesToConfigure) {
        emit messageLogged("Configuring zone: " + zoneName);
        success &= addPort(zoneName, "8080", "tcp");
        success &= addPort(zoneName, "53", "udp");
        success &= addPort(zoneName, "49152-65535", "udp");
    }

    if (success) {
        emit messageLogged("SUCCESS: Firewall ports ensured in all active zones.");
    } else {
        emit messageLogged("WARNING: Some firewall rules could not be applied.");
    }

    return success;
}

bool FirewallManagerLinux::addPort(const QString &zone, const QString &port, const QString &protocol) {
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
    
    // addPort (zone, port, protocol, timeout)
    // timeout 0 = until restart (runtime only)
    QDBusReply<QString> reply = fw.call("addPort", zone, port, protocol, 0);
    
    if (!reply.isValid()) {
        if (reply.error().name() == "org.fedoraproject.FirewallD1.Exception.ALREADY_ENABLED") {
            emit messageLogged(QString("Port %1/%2 already open.").arg(port, protocol));
            return true;
        }
        emit messageLogged(QString("ERROR: Failed to open %1/%2: %3").arg(port, protocol, reply.error().message()));
        return false;
    }

    emit messageLogged(QString("Opened port %1/%2.").arg(port, protocol));
    return true;
}
