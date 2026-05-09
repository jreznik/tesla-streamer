#include "FirewallManagerLinux.h"
#include <QtDBus/QDBusMessage>
#include <QtDBus/QDBusReply>
#include <QtDBus/QDBusConnectionInterface>
#include <QProcess>
#include <QDebug>

#define FIREWALLD_SERVICE "org.fedoraproject.FirewallD1"
#define FIREWALLD_PATH "/org/fedoraproject/FirewallD1"
#define FIREWALLD_IFACE_ZONE "org.fedoraproject.FirewallD1.zone"

FirewallManagerLinux::FirewallManagerLinux(QObject *parent) : IFirewallManager() {
}

FirewallManagerLinux::~FirewallManagerLinux() {
}

bool FirewallManagerLinux::configureFirewall() {
    emit messageLogged("Detecting firewall backend...");
    bool hasFirewallD = QDBusConnection::systemBus().interface()->isServiceRegistered(FIREWALLD_SERVICE);
    
    if (hasFirewallD) {
        emit messageLogged("Using firewalld D-Bus...");
        QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_SERVICE, QDBusConnection::systemBus());
        QDBusMessage msg = QDBusMessage::createMethodCall(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, "getActiveZones");
        QDBusMessage reply = QDBusConnection::systemBus().call(msg);
        
        QStringList zones;
        if (reply.type() != QDBusMessage::ErrorMessage) {
            const QDBusArgument arg = reply.arguments().at(0).value<QDBusArgument>();
            QMap<QString, QStringList> activeZones;
            arg >> activeZones;
            zones = activeZones.keys();
        }
        if (zones.isEmpty()) {
            QDBusReply<QString> defaultZone = fw.call("getDefaultZone");
            if (defaultZone.isValid()) zones << defaultZone.value();
        }
        if (!zones.contains("nm-shared")) zones << "nm-shared";
        if (!zones.contains("public")) zones << "public";

        for (const QString &zoneName : zones) {
            emit messageLogged("Enabling redirection in zone: " + zoneName);
            addPort(zoneName, "8080", "tcp"); 
            addPort(zoneName, "5353", "udp"); 
            addPort(zoneName, "49152-65535", "udp"); 
            addRichRule(zoneName, "rule family=\"ipv4\" forward-port port=\"53\" protocol=\"udp\" to-port=\"5353\"");
            addRichRule(zoneName, "rule family=\"ipv4\" forward-port port=\"80\" protocol=\"tcp\" to-port=\"8080\"");
            fw.call("addMasquerade", zoneName, 0);
        }
        return true;
    } else {
        emit messageLogged("firewalld not found. FALLING BACK TO IPTABLES (STREAMS/STEAMDECK)...");
        bool ok = true;
        // Interception Rules
        ok &= runCommand("sudo", {"iptables", "-t", "nat", "-I", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", "5353"});
        ok &= runCommand("sudo", {"iptables", "-t", "nat", "-I", "PREROUTING", "-p", "tcp", "--dport", "80", "-j", "REDIRECT", "--to-ports", "8080"});
        
        // ACCEPT Rules
        ok &= runCommand("sudo", {"iptables", "-I", "INPUT", "-p", "udp", "--dport", "5353", "-j", "ACCEPT"});
        ok &= runCommand("sudo", {"iptables", "-I", "INPUT", "-p", "tcp", "--dport", "8080", "-j", "ACCEPT"});
        ok &= runCommand("sudo", {"iptables", "-I", "INPUT", "-p", "udp", "--dport", "49152:65535", "-j", "ACCEPT"});

        // DEBUG LOGGING Rules (See with 'dmesg -w')
        runCommand("sudo", {"iptables", "-t", "nat", "-I", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "LOG", "--log-prefix", "[TESLA-DNS] "});
        runCommand("sudo", {"iptables", "-t", "nat", "-I", "PREROUTING", "-p", "tcp", "--dport", "80", "-j", "LOG", "--log-prefix", "[TESLA-HTTP] "});

        if (ok) {
            emit messageLogged("SUCCESS: iptables redirection active. Monitor with 'dmesg -w | grep TESLA'");
        }
        return ok;
    }
}

bool FirewallManagerLinux::cleanupFirewall() {
    emit messageLogged("Cleaning up firewall...");
    if (QDBusConnection::systemBus().interface()->isServiceRegistered(FIREWALLD_SERVICE)) {
        QDBusInterface fw(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, QDBusConnection::systemBus());
        QDBusMessage msg = QDBusMessage::createMethodCall(FIREWALLD_SERVICE, FIREWALLD_PATH, FIREWALLD_IFACE_ZONE, "getActiveZones");
        QDBusMessage reply = QDBusConnection::systemBus().call(msg);
        QStringList zones;
        if (reply.type() != QDBusMessage::ErrorMessage) {
            const QDBusArgument arg = reply.arguments().at(0).value<QDBusArgument>();
            QMap<QString, QStringList> activeZones;
            arg >> activeZones;
            zones = activeZones.keys();
        }
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
    } else {
        runCommand("sudo", {"iptables", "-t", "nat", "-D", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", "5353"});
        runCommand("sudo", {"iptables", "-t", "nat", "-D", "PREROUTING", "-p", "tcp", "--dport", "80", "-j", "REDIRECT", "--to-ports", "8080"});
        runCommand("sudo", {"iptables", "-D", "INPUT", "-p", "udp", "--dport", "5353", "-j", "ACCEPT"});
        runCommand("sudo", {"iptables", "-D", "INPUT", "-p", "tcp", "--dport", "8080", "-j", "ACCEPT"});
        runCommand("sudo", {"iptables", "-D", "INPUT", "-p", "udp", "--dport", "49152:65535", "-j", "ACCEPT"});
        runCommand("sudo", {"iptables", "-t", "nat", "-D", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "LOG", "--log-prefix", "[TESLA-DNS] "});
        runCommand("sudo", {"iptables", "-t", "nat", "-D", "PREROUTING", "-p", "tcp", "--dport", "80", "-j", "LOG", "--log-prefix", "[TESLA-HTTP] "});
    }
    emit messageLogged("Firewall cleaned.");
    return true;
}

bool FirewallManagerLinux::runCommand(const QString &cmd, const QStringList &args) {
    QProcess proc;
    proc.start(cmd, args);
    return proc.waitForFinished() && proc.exitCode() == 0;
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
