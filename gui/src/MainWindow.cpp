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

#include "MainWindow.h"
#include "NetworkManagerLinux.h"
#include <QCoreApplication>
#include <QDir>
#include <QIcon>
#include <QFile>
#include <QNetworkRequest>
#include <QDesktopServices>
#include <QUrl>
#include <QJsonObject>
#include <QJsonDocument>
#include <QSpacerItem>

MainWindow::MainWindow(QWidget *parent) : QMainWindow(parent), 
    m_process(new QProcess(this)), 
    m_networkManager(new QNetworkAccessManager(this)),
    m_netManager(new NetworkManagerLinux(this)) 
{
    setupUI();
    setupTray();

    connect(m_process, &QProcess::readyReadStandardOutput, this, &MainWindow::readOutput);
    connect(m_process, &QProcess::readyReadStandardError, this, &MainWindow::readOutput);
    connect(m_networkManager, &QNetworkAccessManager::finished, this, &MainWindow::handleControlReply);
    connect(m_netManager, &INetworkManager::hotspotStateChanged, this, &MainWindow::onHotspotStateChanged);
    connect(m_netManager, &INetworkManager::messageLogged, m_logArea, &QTextEdit::append);
}

MainWindow::~MainWindow() {
    stopServer();
}

void MainWindow::setupUI() {
    setWindowTitle("Tesla Streamer Control");
    setWindowIcon(QIcon(":/io.github.jreznik.TeslaStreamer.svg"));
    
    QWidget *central = new QWidget(this);
    QVBoxLayout *mainLayout = new QVBoxLayout(central);
    mainLayout->setSpacing(10);
    mainLayout->setContentsMargins(15, 15, 15, 15);
    // Force window to follow the size of the contents exactly
    mainLayout->setSizeConstraint(QLayout::SetFixedSize);

    // --- Row 1: Streaming Configuration (Horizontal) ---
    m_settingsGroup = new QGroupBox("Streaming Configuration", this);
    QHBoxLayout *settingsLayout = new QHBoxLayout(m_settingsGroup);
    settingsLayout->setSpacing(15);
    
    settingsLayout->addWidget(new QLabel("Profile:"));
    m_profileCombo = new QComboBox();
    m_profileCombo->addItems({"latency", "balanced", "quality"});
    settingsLayout->addWidget(m_profileCombo);

    settingsLayout->addWidget(new QLabel("Display:"));
    m_displayCombo = new QComboBox();
    m_displayCombo->addItem("Scale to Fit", "fit");
    m_displayCombo->addItem("Stretch to Fill", "stretch");
    m_displayCombo->addItem("Native Size", "native");
    settingsLayout->addWidget(m_displayCombo);

    m_statsCheckbox = new QCheckBox("Overlay");
    settingsLayout->addWidget(m_statsCheckbox);

    connect(m_profileCombo, &QComboBox::currentTextChanged, this, &MainWindow::updateConfig);
    connect(m_displayCombo, &QComboBox::currentTextChanged, this, &MainWindow::updateConfig);
    connect(m_statsCheckbox, &QCheckBox::stateChanged, this, &MainWindow::updateConfig);
    mainLayout->addWidget(m_settingsGroup);

    // --- Row 2: Action Buttons (Horizontal) ---
    QHBoxLayout *actionLayout = new QHBoxLayout();
    actionLayout->setSpacing(10);

    m_startBtn = new QPushButton("Start Server");
    m_startBtn->setFixedHeight(35);
    m_startBtn->setMinimumWidth(120);
    actionLayout->addWidget(m_startBtn);
    connect(m_startBtn, &QPushButton::clicked, this, &MainWindow::toggleServer);

    m_hotspotBtn = new QPushButton("Start Hotspot");
    m_hotspotBtn->setFixedHeight(35);
    m_hotspotBtn->setMinimumWidth(120);
    actionLayout->addWidget(m_hotspotBtn);
    connect(m_hotspotBtn, &QPushButton::clicked, this, &MainWindow::toggleHotspot);

    m_reselectBtn = new QPushButton("Reselect Source");
    m_reselectBtn->setFixedHeight(35);
    m_reselectBtn->setEnabled(false);
    actionLayout->addWidget(m_reselectBtn);
    connect(m_reselectBtn, &QPushButton::clicked, this, &MainWindow::reselectSource);

    mainLayout->addLayout(actionLayout);

    // --- Row 3: Status Information ---
    m_controlGroup = new QGroupBox("Status", this);
    QVBoxLayout *statusLayout = new QVBoxLayout(m_controlGroup);

    QHBoxLayout *statusHeader = new QHBoxLayout();
    m_statusLabel = new QLabel("Server: <b>Stopped</b>");
    statusHeader->addWidget(m_statusLabel);
    
    m_urlLabel = new QLabel("http://localhost:8080");
    m_urlLabel->setTextInteractionFlags(Qt::TextBrowserInteraction);
    m_urlLabel->setVisible(false);
    statusHeader->addWidget(m_urlLabel);

    m_openBrowserBtn = new QPushButton("Open Browser");
    m_openBrowserBtn->setVisible(false);
    m_openBrowserBtn->setFlat(true);
    m_openBrowserBtn->setStyleSheet("color: blue; text-decoration: underline;");
    statusHeader->addWidget(m_openBrowserBtn);
    connect(m_openBrowserBtn, &QPushButton::clicked, [this]() {
        QDesktopServices::openUrl(QUrl("http://localhost:8080"));
    });
    statusHeader->addStretch();
    statusLayout->addLayout(statusHeader);

    m_hotspotInfoLabel = new QLabel("Wi-Fi: <b>TeslaStreamer</b> (pw: <b>tesla123</b>)");
    m_hotspotInfoLabel->setVisible(false);
    statusLayout->addWidget(m_hotspotInfoLabel);
    
    mainLayout->addWidget(m_controlGroup);

    // --- Row 4: Log Toggle and Area ---
    QHBoxLayout *logToggleLayout = new QHBoxLayout();
    logToggleLayout->addStretch();
    m_toggleLogsBtn = new QPushButton("Show Logs");
    m_toggleLogsBtn->setFlat(true);
    logToggleLayout->addWidget(m_toggleLogsBtn);
    connect(m_toggleLogsBtn, &QPushButton::clicked, this, &MainWindow::toggleLogs);
    mainLayout->addLayout(logToggleLayout);

    m_logArea = new QTextEdit();
    m_logArea->setReadOnly(true);
    m_logArea->setFontFamily("Monospace");
    m_logArea->setFontPointSize(8);
    m_logArea->setVisible(false);
    m_logArea->setFixedSize(600, 150); 
    mainLayout->addWidget(m_logArea);

    setCentralWidget(central);
}

void MainWindow::setupTray() {
    m_trayIcon = new QSystemTrayIcon(this);
    m_trayIcon->setIcon(QIcon(":/tesla-streamer-tray.svg"));
    m_trayIcon->setToolTip("Tesla Streamer");

    m_trayMenu = new QMenu(this);
    QAction *showAction = m_trayMenu->addAction("Show Settings");
    connect(showAction, &QAction::triggered, this, &MainWindow::show);

    QAction *reselectAction = m_trayMenu->addAction("Reselect Window/Screen");
    connect(reselectAction, &QAction::triggered, this, &MainWindow::reselectSource);

    m_trayMenu->addSeparator();
    QAction *quitAction = m_trayMenu->addAction("Quit");
    connect(quitAction, &QAction::triggered, qApp, &QCoreApplication::quit);

    m_trayIcon->setContextMenu(m_trayMenu);
    m_trayIcon->show();

    connect(m_trayIcon, &QSystemTrayIcon::activated, this, &MainWindow::trayIconActivated);
}

void MainWindow::toggleServer() {
    if (m_process->state() == QProcess::Running) {
        stopServer();
    } else {
        startServer();
    }
}

void MainWindow::toggleHotspot() {
    if (m_netManager->isHotspotActive()) {
        m_netManager->stopHotspot();
    } else {
        m_logArea->append("Requesting Hotspot start...");
        if (!m_netManager->startHotspot("TeslaStreamer", "tesla123")) {
            m_logArea->append("ERROR: Failed to initiate hotspot startup");
        }
    }
}

void MainWindow::toggleLogs() {
    bool visible = m_logArea->isVisible();
    m_logArea->setVisible(!visible);
    m_toggleLogsBtn->setText(visible ? "Show Logs" : "Hide Logs");
}

void MainWindow::onHotspotStateChanged(bool active) {
    if (active) {
        m_hotspotBtn->setText("Stop Hotspot");
        m_hotspotInfoLabel->setVisible(true);
        m_urlLabel->setText("http://play.tesla.stream:8080");
    } else {
        m_hotspotBtn->setText("Start Hotspot");
        m_hotspotInfoLabel->setVisible(false);
        m_urlLabel->setText("http://localhost:8080");
    }
}

void MainWindow::startServer() {
    QString program = QCoreApplication::applicationDirPath() + "/tesla-streamer";
    QString workingDir = QCoreApplication::applicationDirPath();
    
    // Flatpak support: check standard location for assets
    QString flatpakAssets = "/app/share/tesla-streamer";
    if (QFile::exists(flatpakAssets + "/static")) {
        workingDir = flatpakAssets;
    } else {
        // Local dev support
        if (!QFile::exists(program)) {
            program = QCoreApplication::applicationDirPath() + "/../tesla-streamer";
            workingDir = QCoreApplication::applicationDirPath() + "/../";
        }
        if (!QFile::exists(program)) {
            program = QCoreApplication::applicationDirPath() + "/../../tesla-streamer";
            workingDir = QCoreApplication::applicationDirPath() + "/../../";
        }
    }
    
    QDir dir(workingDir);
    workingDir = dir.absolutePath();

    QStringList arguments;
    arguments << "--profile" << m_profileCombo->currentText();
    
    m_logArea->append(QString("--- Starting backend: %1 ---").arg(program));
    m_logArea->append(QString("--- Working directory: %1 ---").arg(workingDir));
    
    m_process->setWorkingDirectory(workingDir);
    m_process->start(program, arguments);
    
    if (m_process->waitForStarted()) {
        m_startBtn->setText("Stop Server");
        m_statusLabel->setText("Server: <b>Running</b>");
        m_reselectBtn->setEnabled(true);
        m_urlLabel->setVisible(true);
        m_openBrowserBtn->setVisible(true);
        
        updateConfig();
    } else {
        m_logArea->append("ERROR: Failed to start backend process: " + m_process->errorString());
    }
}

void MainWindow::stopServer() {
    if (m_process->state() == QProcess::Running) {
        m_process->terminate();
        if (!m_process->waitForFinished(3000)) {
            m_process->kill();
        }
        m_startBtn->setText("Start Server");
        m_statusLabel->setText("Server: <b>Stopped</b>");
        m_reselectBtn->setEnabled(false);
        m_urlLabel->setVisible(false);
        m_openBrowserBtn->setVisible(false);
        m_logArea->append("--- Server Stopped ---");
    }
}

void MainWindow::updateConfig() {
    if (m_process->state() == QProcess::Running) {
        sendConfig();
    }
}

void MainWindow::sendConfig() {
    QJsonObject obj;
    obj["profile"] = m_profileCombo->currentText();
    obj["display"] = m_displayCombo->currentData().toString();
    obj["show_stats"] = m_statsCheckbox->isChecked();
    
    QNetworkRequest request(QUrl("http://localhost:8080/api/config"));
    request.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");
    
    m_networkManager->post(request, QJsonDocument(obj).toJson());
    m_logArea->append("Sent config update...");
}

void MainWindow::readOutput() {
    m_logArea->append(m_process->readAllStandardOutput());
    m_logArea->append(m_process->readAllStandardError());
}

void MainWindow::trayIconActivated(QSystemTrayIcon::ActivationReason reason) {
    if (reason == QSystemTrayIcon::Trigger) {
        if (isVisible()) {
            hide();
        } else {
            show();
            raise();
            activateWindow();
        }
    }
}

void MainWindow::reselectSource() {
    if (m_process->state() != QProcess::Running) {
        m_logArea->append("Warning: Server is not running. Cannot reselect.");
        return;
    }
    
    m_networkManager->get(QNetworkRequest(QUrl("http://localhost:8080/api/reselect")));
}

void MainWindow::handleControlReply(QNetworkReply *reply) {
    if (reply->error() != QNetworkReply::NoError) {
        if (m_process->state() == QProcess::Running) {
            m_logArea->append("Control API Error: " + reply->errorString());
        }
    } else {
        m_logArea->append("Control API: Request successful");
    }
    reply->deleteLater();
}
