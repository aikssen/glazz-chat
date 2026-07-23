import { readFile, readdir } from "node:fs/promises";
import path from "node:path";

import { Parser } from "@asyncapi/parser";
import Ajv from "ajv";
import addFormats from "ajv-formats";

const contractPath = "packages/contracts/websocket.asyncapi.yaml";
const fixtureDirectory = "packages/contracts/fixtures/ws";
const parser = new Parser();
const source = await readFile(contractPath, "utf8");
const { document, diagnostics } = await parser.parse(source);
const errors = diagnostics.filter((diagnostic) => diagnostic.severity === 0);

if (!document || errors.length > 0) {
  for (const error of errors) {
    console.error(`${error.path.join(".")}: ${error.message}`);
  }
  throw new Error(`AsyncAPI contract has ${errors.length} error(s)`);
}

const ajv = new Ajv({ allErrors: true, strict: false });
addFormats(ajv);
const validators = new Map(
  document
    .components()
    .messages()
    .all()
    .map((message) => {
      const payload = message.payload();
      if (!payload) {
        throw new Error(`Message ${message.name()} has no payload schema`);
      }
      return [message.name(), ajv.compile(payload.json())];
    }),
);

const fixtureNames = (await readdir(fixtureDirectory))
  .filter((name) => name.endsWith(".json"))
  .sort();
let eventCount = 0;

for (const fixtureName of fixtureNames) {
  const fixturePath = path.join(fixtureDirectory, fixtureName);
  const events = JSON.parse(await readFile(fixturePath, "utf8"));
  if (!Array.isArray(events)) {
    throw new Error(`${fixturePath} must contain an array of events`);
  }

  for (const [index, event] of events.entries()) {
    const validate = validators.get(event?.type);
    if (!validate) {
      throw new Error(`${fixturePath}[${index}] has unknown event type ${event?.type}`);
    }
    if (!validate(event)) {
      throw new Error(
        `${fixturePath}[${index}] does not match ${event.type}: ${ajv.errorsText(validate.errors)}`,
      );
    }
    eventCount += 1;
  }
}

console.log(`Validated ${eventCount} events across ${fixtureNames.length} WebSocket fixtures.`);
