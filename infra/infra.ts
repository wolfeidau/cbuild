#!/usr/bin/env node
import 'source-map-support/register';
import { App } from "@aws-cdk/core"
import { CodeBuilderStack } from './lib/infra-stack';

const app = new App();
new CodeBuilderStack(app, 'BuilderStack-dev-master');
