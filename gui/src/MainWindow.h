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

#ifndef MAINWINDOW_H
#define MAINWINDOW_H

#include <QMainWindow>
#include <QProcess>
#include <QSystemTrayIcon>
#include <QMenu>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QComboBox>
#include <QPushButton>
#include <QTextEdit>
#include <QLabel>
#include <QGroupBox>
#include <QCheckBox>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include "INetworkManager.h"
#include "IFirewallManager.h"

class MainWindow : public QMainWindow {
    Q_OBJECT

public:
    MainWindow(QWidget *parent = nullptr);
    ~MainWindow();

private slots:
    void toggleServer();
    void readOutput();
    void trayIconActivated(QSystemTrayIcon::ActivationReason reason);
    void reselectSource();
    void updateConfig();
    void toggleHotspot();
    void toggleLogs();
    void handleControlReply(QNetworkReply *reply);
    void onHotspotStateChanged(bool active);

private:
    void setupUI();
    void setupTray();
    void startServer();
    void stopServer();
    void sendConfig();

    QProcess *m_process;
    QSystemTrayIcon *m_trayIcon;
    QMenu *m_trayMenu;

    INetworkManager *m_netManager;
    IFirewallManager *m_fwManager;

    // Settings Group
    QGroupBox *m_settingsGroup;
    QComboBox *m_profileCombo;
    QComboBox *m_displayCombo;
    QCheckBox *m_statsCheckbox;

    // Network Group
    QGroupBox *m_networkGroup;
    QComboBox *m_hotspotDeviceCombo;
    QPushButton *m_hotspotBtn;
    QLabel *m_hotspotInfoLabel;

    // Control Group
    QGroupBox *m_controlGroup;
    QPushButton *m_startBtn;
    QPushButton *m_reselectBtn;
    QPushButton *m_openBrowserBtn;
    QLabel *m_statusLabel;
    QLabel *m_urlLabel;

    // Log Area
    QPushButton *m_toggleLogsBtn;
    QTextEdit *m_logArea;

    QNetworkAccessManager *m_networkManager;
};

#endif
