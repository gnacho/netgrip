import test from "node:test";
import assert from "node:assert/strict";
import { compressRanges, portNum } from "./vlanRanges.ts";

test("compressRanges vacío devuelve placeholder", () => {
  assert.equal(compressRanges([]), "-");
});

test("compressRanges comprime consecutivos", () => {
  assert.equal(compressRanges([1, 2, 3, 5, 6, 7, 8]), "1-3, 5-8");
});

test("compressRanges aísla no consecutivos", () => {
  assert.equal(compressRanges([1, 3, 5]), "1, 3, 5");
});

test("compressRanges ordena y deduplica", () => {
  assert.equal(compressRanges([8, 1, 2, 8, 3]), "1-3, 8");
});

test("compressRanges acepta desorden y solitarios", () => {
  assert.equal(compressRanges([52, 40, 41]), "40-41, 52");
});

test("portNum extrae el número final", () => {
  assert.equal(portNum("lan20"), 20);
  assert.equal(portNum("lan1"), 1);
  assert.ok(Number.isNaN(portNum("wan")));
});
