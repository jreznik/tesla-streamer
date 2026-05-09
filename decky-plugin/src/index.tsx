/**
 * Tesla Streamer - High-performance screen streaming for Tesla browsers
 * Copyright (C) 2026 Jaroslav Reznik
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

import {
  definePlugin,
  ServerAPI,
  StaticContext,
  Dropdown,
  ButtonItem,
  ToggleField,
  PanelSection,
  PanelSectionRow,
} from "@decky/ui";
import { FaCar, FaExclamationTriangle } from "react-icons/fa";
import { useState, useEffect } from "react";

const Content = ({ serverAPI }: { serverAPI: ServerAPI }) => {
  const [status, setStatus] = useState<any>({ installed: true, server_running: false, hotspot_active: false });
  const [profile, setProfile] = useState("latency");
  const [display, setDisplay] = useState("fit");
  const [showStats, setShowStats] = useState(false);

  const updateStatus = async () => {
    const res = await serverAPI.callPluginMethod("get_status", {});
    if (res.success) setStatus(res.result);
  };

  useEffect(() => {
    updateStatus();
    const interval = setInterval(updateStatus, 5000);
    return () => clearInterval(interval);
  }, []);

  const toggleServer = async () => {
    if (status.server_running) {
      await serverAPI.callPluginMethod("stop_server", {});
    } else {
      await serverAPI.callPluginMethod("start_server", { profile });
    }
    updateStatus();
  };

  const toggleHotspot = async () => {
    if (status.hotspot_active) {
      await serverAPI.callPluginMethod("stop_hotspot", {});
    } else {
      await serverAPI.callPluginMethod("start_hotspot", {});
    }
    updateStatus();
  };

  const sendConfig = async (newConfig: any) => {
    await serverAPI.callPluginMethod("update_config", {
      profile,
      display,
      show_stats: showStats,
      ...newConfig
    });
  };

  if (!status.installed) {
    return (
      <PanelSection title="Installation Required">
        <PanelSectionRow>
          <div style={{ display: "flex", alignItems: "center", gap: "10px", color: "#ffc107" }}>
            <FaExclamationTriangle />
            <span>Flatpak not found</span>
          </div>
        </PanelSectionRow>
        <PanelSectionRow>
          <p style={{ fontSize: "0.9em", color: "#ccc" }}>
            Please install the "Tesla Streamer" application via Desktop Mode or Flatpak before using this plugin.
          </p>
        </PanelSectionRow>
      </PanelSection>
    );
  }

  return (
    <>
      <PanelSection title="Hotspot (Tesla Mode)">
        <PanelSectionRow>
          <ButtonItem layout="below" onClick={toggleHotspot}>
            {status.hotspot_active ? "Stop Tesla Mode" : "Start Tesla Mode"}
          </ButtonItem>
        </PanelSectionRow>
        {status.hotspot_active && (
          <PanelSectionRow>
            <div style={{ fontSize: "0.8em", color: "#aaa" }}>
              Connect Tesla to: <b>TeslaStreamer</b><br />
              URL: <b>http://play.tesla.stream:8080</b>
            </div>
          </PanelSectionRow>
        )}
      </PanelSection>

      <PanelSection title="Streaming">
        <PanelSectionRow>
          <Dropdown
            label="Quality Profile"
            rgOptions={[
              { data: "latency", label: "Latency" },
              { data: "balanced", label: "Balanced" },
              { data: "quality", label: "Quality" },
            ]}
            selectedOption={profile}
            onChange={(opt: any) => {
              setProfile(opt.data);
              sendConfig({ profile: opt.data });
            }}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <Dropdown
            label="Display Mode"
            rgOptions={[
              { data: "fit", label: "Scale to Fit" },
              { data: "stretch", label: "Stretch to Fill" },
              { data: "native", label: "Native Size" },
            ]}
            selectedOption={display}
            onChange={(opt: any) => {
              setDisplay(opt.data);
              sendConfig({ display: opt.data });
            }}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <ToggleField
            label="Performance Overlay"
            checked={showStats}
            onChange={(val) => {
              setShowStats(val);
              sendConfig({ show_stats: val });
            }}
          />
        </PanelSectionRow>
        <PanelSectionRow>
          <ButtonItem 
            layout="below" 
            onClick={toggleServer}
            highlight={status.server_running}
          >
            {status.server_running ? "Stop Server" : "Start Server"}
          </ButtonItem>
        </PanelSectionRow>
        {status.server_running && (
          <PanelSectionRow>
            <ButtonItem layout="below" onClick={() => serverAPI.callPluginMethod("reselect_source", {})}>
              Reselect Source
            </ButtonItem>
          </PanelSectionRow>
        )}
      </PanelSection>
    </>
  );
};

export default definePlugin((serverAPI: ServerAPI, context: StaticContext) => {
  return {
    title: <div style={{ color: "#e11d48", fontWeight: "bold" }}>Tesla Streamer</div>,
    content: <Content serverAPI={serverAPI} />,
    icon: <FaCar />,
    onDismount() {
      // Cleanup happens in backend
    },
  };
});
