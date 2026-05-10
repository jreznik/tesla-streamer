#ifndef IFIREWALLMANAGER_H
#define IFIREWALLMANAGER_H

#include <QString>
#include <QObject>

class IFirewallManager : public QObject {
    Q_OBJECT

public:
    virtual ~IFirewallManager() {}

    virtual bool configureFirewall(bool offlineMode) = 0;
    virtual bool cleanupFirewall() = 0;

signals:
    void messageLogged(const QString &msg);
};

#endif
