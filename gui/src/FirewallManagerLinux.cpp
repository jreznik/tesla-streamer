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
    QDBusMessage msg = QDBusMessage::createMethodCall(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, "getActiveZones");
    QDBusMessage reply = QDBusConnection::systemBus().call(msg);
    
    QStringList zonesToConfigure;
    if (reply.type() != QDBusMessage::ErrorMessage) {
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
        
        // 1. Open Base Ports
        success &= addPort(zoneName, "8080", "tcp"); // Signaling
        success &= addPort(zoneName, "5353", "udp"); // Local DNS Spoofer
        success &= addPort(zoneName, "49152-65535", "udp"); // WebRTC Media
        
        // 2. Transparent Redirection (NAT)
        // Redirection allows the car to work without specifying ports
        
        // DNS: 53 -> 5353
        success &= addForward(zoneName, "53", "udp", "5353");
        
        // HTTP: 80 -> 8080
        success &= addForward(zoneName, "80", "tcp", "8080");

        // 3. Enable Masquerading (Required for Forwarding/Bridging)
        QDBusReply<void> masqReply = fw.call("addMasquerade", zoneName, 0);
        if (masqReply.isValid()) {
            emit messageLogged("Masquerading enabled for " + zoneName);
        }
    }

    if (success) {
        emit messageLogged("SUCCESS: Transparent redirection active.");
    } else {
        emit messageLogged("WARNING: Some firewall rules failed. Check log above.");
    }

    return success;
}

bool FirewallManagerLinux::addPort(const QString &zone, const QString &port, const QString &protocol) {
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
    
    // addPort (zone, port, protocol, timeout)
    QDBusReply<QString> reply = fw.call("addPort", zone, port, protocol, 0);
    
    if (!reply.isValid()) {
        if (reply.error().name() == "org.fedoraproject.FirewallD1.Exception.ALREADY_ENABLED") {
            return true;
        }
        emit messageLogged(QString("ERROR: Port %1/%2 failed: %3").arg(port, protocol, reply.error().message()));
        return false;
    }
    return true;
}

bool FirewallManagerLinux::addForward(const QString &zone, const QString &port, const QString &protocol, const QString &toPort) {
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
    
    // addForwardPort (zone, port, protocol, toport, toaddr, timeout)
    // toaddr is empty for local redirection
    QDBusReply<QString> reply = fw.call("addForwardPort", zone, port, protocol, toPort, "", 0);
    
    if (!reply.isValid()) {
        if (reply.error().name() == "org.fedoraproject.FirewallD1.Exception.ALREADY_ENABLED") {
            return true;
        }
        emit messageLogged(QString("ERROR: Redirect %1->%2 failed: %3").arg(port, toPort, reply.error().message()));
        return false;
    }
    emit messageLogged(QString("Redirect active: %1/%2 -> %3").arg(port, protocol, toPort));
    return true;
}
