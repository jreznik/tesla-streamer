#ifndef IFIREWALLMANAGER_H
#define IFIREWALLMANAGER_H

#include <QString>
#include <QObject>

class IFirewallManager : public QObject {
    Q_OBJECT

public:
    virtual ~IFirewallManager() {}

    virtual bool configureFirewall() = 0;

signals:
    void messageLogged(const QString &msg);
};

#endif
