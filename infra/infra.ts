#!/usr/bin/env node
import 'source-map-support/register';
import { App } from "@aws-cdk/core"
import { CodeBuilderStack } from './lib/infra-stack';

const app = new App();

const branch = app.node.tryGetContext("branch");
const stage = app.node.tryGetContext("stage");

new CodeBuilderStack(app, `BuilderStack-${stage}-${branch}`);
