import test from "node:test";
import assert from "node:assert/strict";
import { degradedPorts } from "./degradedPorts.ts";

const port = (name, speed_mbps, supported, autoneg = true) => ({
  name, autoneg, speed_mbps, duplex: "full",
  supported: supported.map((s) => ({ speed_mbps: s, duplex: "full" })),
  eee_supported: false, eee_enabled: false, eee_writable: true,
  mtu: 1500, mtu_max: 1500, mtu_supported: false,
});

test("puerto a 100 Mbps en hardware 1G = degradado", () => {
  const probe = { applicable: true, ports: [port("lan2", 100, [10, 100, 1000])] };
  assert.deepEqual(degradedPorts(probe), [{ name: "lan2", speed: 100, max: 1000 }]);
});

test("puerto a tope de su capacidad no es degradado", () => {
  const probe = { applicable: true, ports: [port("lan1", 1000, [10, 100, 1000])] };
  assert.deepEqual(degradedPorts(probe), []);
});

test("puerto caído (speed 0) no cuenta", () => {
  const probe = { applicable: true, ports: [port("lan3", 0, [10, 100, 1000])] };
  assert.deepEqual(degradedPorts(probe), []);
});

test("sin autoneg no se juzga (velocidad forzada por el admin)", () => {
  const probe = { applicable: true, ports: [port("lan4", 100, [10, 100, 1000], false)] };
  assert.deepEqual(degradedPorts(probe), []);
});

test("probe no aplicable o ausente = sin degradados", () => {
  assert.deepEqual(degradedPorts(undefined), []);
  assert.deepEqual(degradedPorts({ applicable: false, ports: [] }), []);
});

test("varios degradados: todos listados", () => {
  const probe = { applicable: true, ports: [port("lan1", 100, [1000]), port("lan2", 1000, [1000]), port("lan3", 10, [1000])] };
  assert.deepEqual(degradedPorts(probe).map((d) => d.name), ["lan1", "lan3"]);
});
