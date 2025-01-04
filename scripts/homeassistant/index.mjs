// Keys are the native ha entity IDs.
const config = {
    "switch.plug_1": {
        "speech": "office",
        "switch": "0",
        "metric": "ha.plug.office",
        "getter": (state) => state.state,
        "max": 1,
        "mapping": {
            "off": 0,
            "on": 1,
        },
    },
    "switch.plug_corridor": {
        "speech": "corridor",
        "switch": "1",
        "metric": "ha.plug.corridor",
        "getter": (state) => state.state,
        "max": 1,
        "mapping": {
            "off": 0,
            "on": 1,
        },
    },
    "switch.plug_music_room": {
        "speech": "music room",
        "switch": "2",
        "metric": "ha.plug.music_room",
        "getter": (state) => state.state,
        "max": 1,
        "mapping": {
            "off": 0,
            "on": 1,
        },
    },
    "light.square_bulb": {
        "speech": "square light",
        "switch": "3",
        "metric": "ha.light.square_bulb",
        "getter": (state) => state.state,
        "max": 1,
        "mapping": {
            "off": 0,
            "on": 1,
        },
    },
    "light.low_bulb": {
        "speech": "low light",
        "switch": "4",
        "metric": "ha.light.low_bulb",
        "getter": (state) => state.state,
        "max": 1,
        "mapping": {
            "off": 0,
            "on": 1,
        },
    },
    "light.high_bulb": {
        "speech": "high light",
        "switch": "5",
        "metric": "ha.light.high_bulb",
        "getter": (state) => state.state,
        "max": 1,
        "mapping": {
            "off": 0,
            "on": 1,
        },
    },
    "person.chris": {
        "metric": "ha.chris.home",
        "getter": (state) => state.state,
        "max": 1,
        "mapping": {
            "not_home": 0,
            "home": 1,
        },
    },
    "device_tracker.ginger_tracker": {
        "metric": "ha.ginger.home",
        "getter": (state) => state.state,
        "max": 1,
        "mapping": {
            "not_home": 0,
            "home": 1,
        },
    },
    "weather.forecast_home": {
        "metric": "ha.weather.temp",
        "getter": (state) => Math.round(state.attributes.temperature),
        "max": 40,
    },
    "sensor.dyson_temperature": {
        "metric": "ha.office.temp",
        "getter": (state) => Math.round(state.state),
        "max": 40,
    },
    "sensor.e208_battery_level": {
        "metric": "ha.car.level",
        "getter": (state) => Math.round(state.state/10),
        "max": 10,
    },
    "sensor.e208_charging_status": {
        "metric": "ha.car.status",
        "getter": (state) => state.state,
        "max": 10,
        "mapping": {
            "Disconnected": 0,
            "InProgress": 1,
        },
    },
};

import {
  Auth,
  callService,
  createConnection,
  subscribeEntities,
  createLongLivedTokenAuth,
} from "home-assistant-js-websocket";
import { createInterface } from "node:readline";

let connection;

// Will be printed in blink11 output
const log = console.error

// Send message to blink11
const emit = console.log

const start = (args) => {
  const epochMs = args[0];
  log("START", epochMs);
};

const stop = (args) =>  log("STOP");

const memory = (args) => log("MEMORY", args);

const tick = (args) => {
  const epochMs = args[0];
  // log("TICK", epochMs);
};

// Handle switch event
const event = (args) => {
  const sw = args[0];
  const on = args[1] == "true";
  log(`EVENT sw=${sw} on=${on}`);
  for (const [id, cfg] of Object.entries(config)) {
      const cfgSwitch = cfg["switch"];
      if (!cfgSwitch || cfgSwitch != sw) continue;
      let service = on ? "turn_on" : "turn_off";
      const speech = cfg["speech"];
      if (speech) {
          const nagThresholdSeconds = 5;
          const lastSpoke = cfg["lastSpoke"] || 0;
          const now = Date.now();
          if (now - lastSpoke > nagThresholdSeconds*1000) {
              emit("sound tts:", speech);
              cfg["lastSpoke"] = now;
          }
      }
      callService(connection, "homeassistant", service, {
        entity_id: id,
      });
  }
};

// Parses messages from blink11 and calls handlers
const eventloop = async () => {
    for await (const line of createInterface({ input: process.stdin })) {
      // log("line:", line);
      const tokens = line.split(/ +/);
      const type = tokens[0];
      let handler
      if (["start", "stop", "tick", "memory", "event"].includes(type)) {
        handler = eval(type);
      }
      if (handler) {
        handler(tokens.slice(1));
      }
    }
};

const handleUpdate = async (cfg, state) => {
    // log("state:", state);
    const metric = cfg["metric"];
    const nativeVal = cfg["getter"](state);
    let val = nativeVal;
    const mapping = cfg["mapping"];
    if (mapping) val = cfg["mapping"][nativeVal];
    const max = cfg["max"] || val;
    emit("metric", metric, val, max);
};

const handleUpdates = (states) => {
    try{
        for (const [entityID, entityCfg] of Object.entries(config)) {
            const entityState = states[entityID]
            if (entityState) {
                handleUpdate(entityCfg, entityState);
            }
        }
    }catch(e){
        log("handleUpdates error:", e);
    }
};

(async () => {
  try {
      const auth = createLongLivedTokenAuth(
          process.env.HASS_SERVER,
          process.env.HASS_TOKEN,
      );
      connection = await createConnection({ auth });
      eventloop();
      subscribeEntities(connection, handleUpdates);
  } catch(e) {
      log("main error:", e);
  }
})()
