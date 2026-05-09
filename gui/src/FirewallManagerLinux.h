#ifndef FIREWALLMANAGERLINUX_H
#define FIREWALLMANAGERLINUX_H

#include "IFirewallManager.h"
#include <QtDBus/QDBusInterface>

class FirewallManagerLinux : public IFirewallManager {
    Q_OBJECT

public:
    FirewallManagerLinux(QObject *parent = nullptr);
    ~FirewallManagerLinux();

    bool configureFirewall() override;

private:
    bool addPort(const QString &zone, const QString &port, const QString &protocol);
    bool addRichRule(const QString &zone, const QString &rule);
};

#endif
