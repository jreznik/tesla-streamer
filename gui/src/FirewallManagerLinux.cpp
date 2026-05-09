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
    emit messageLogged("Setting up firewall redirection...");
    
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_SERVICE, QDBusConnection::systemBus());
    if (!fw.isValid()) {
        emit messageLogged("ERROR: firewalld is not reachable. Redirects will not work.");
        return false;
    }

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
        QDBusReply<QString> defaultZone = fw.call("getDefaultZone");
        if (defaultZone.isValid()) zonesToConfigure << defaultZone.value();
    }

    // Always include common hotspot zones
    if (!zonesToConfigure.contains("nm-shared")) zonesToConfigure << "nm-shared";
    if (!zonesToConfigure.contains("public")) zonesToConfigure << "public";

    bool success = true;
    for (const QString &zoneName : zonesToConfigure) {
        emit messageLogged("Configuring zone: " + zoneName);
        // Base Ports
        success &= addPort(zoneName, "8080", "tcp"); 
        success &= addPort(zoneName, "5353", "udp"); 
        success &= addPort(zoneName, "49152-65535", "udp"); 
        
        // DNS & HTTP Hijacking
        success &= addRichRule(zoneName, "rule family=\"ipv4\" forward-port port=\"53\" protocol=\"udp\" to-port=\"5353\"");
        success &= addRichRule(zoneName, "rule family=\"ipv4\" forward-port port=\"80\" protocol=\"tcp\" to-port=\"8080\"");

        // NAT Masquerade
        fw.call("addMasquerade", zoneName, 0);
    }

    if (success) {
        emit messageLogged("SUCCESS: Transparent redirection active.");
    }
    return success;
}

bool FirewallManagerLinux::cleanupFirewall() {
    emit messageLogged("Cleaning up firewall rules...");
    
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
    if (!fw.isValid()) return false;

    QDBusMessage msg = QDBusMessage::createMethodCall(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, "getActiveZones");
    QDBusMessage reply = QDBusConnection::systemBus().call(msg);
    
    QStringList zones;
    if (reply.type() != QDBusMessage::ErrorMessage) {
        const QDBusArgument arg = reply.arguments().at(0).value<QDBusArgument>();
        QMap<QString, QStringList> activeZones;
        arg >> activeZones;
        zones = activeZones.keys();
    }
    
    // Explicitly cleanup these too
    if (!zones.contains("nm-shared")) zones << "nm-shared";
    if (!zones.contains("public")) zones << "public";

    for (const QString &zoneName : zones) {
        removePort(zoneName, "8080", "tcp");
        removePort(zoneName, "5353", "udp");
        removePort(zoneName, "49152-65535", "udp");
        removeRichRule(zoneName, "rule family=\"ipv4\" forward-port port=\"53\" protocol=\"udp\" to-port=\"5353\"");
        removeRichRule(zoneName, "rule family=\"ipv4\" forward-port port=\"80\" protocol=\"tcp\" to-port=\"8080\"");
        fw.call("removeMasquerade", zoneName);
    }

    emit messageLogged("Firewall rules removed.");
    return true;
}

bool FirewallManagerLinux::addPort(const QString &zone, const QString &port, const QString &protocol) {
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
    QDBusReply<QString> reply = fw.call("addPort", zone, port, protocol, 0);
    return reply.isValid() || reply.error().name() == "org.fedoraproject.FirewallD1.Exception.ALREADY_ENABLED";
}

bool FirewallManagerLinux::removePort(const QString &zone, const QString &port, const QString &protocol) {
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
    QDBusReply<QString> reply = fw.call("removePort", zone, port, protocol);
    return reply.isValid();
}

bool FirewallManagerLinux::addRichRule(const QString &zone, const QString &rule) {
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
    QDBusReply<QString> reply = fw.call("addRichRule", zone, rule, 0);
    return reply.isValid() || reply.error().name() == "org.fedoraproject.FirewallD1.Exception.ALREADY_ENABLED";
}

bool FirewallManagerLinux::removeRichRule(const QString &zone, const QString &rule) {
    QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
    QDBusReply<QString> reply = fw.call("removeRichRule", zone, rule);
    return reply.isValid();
}
