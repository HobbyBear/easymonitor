#!/bin/bash
nohup  /program/webapp &
node_exporter --collector.processes
