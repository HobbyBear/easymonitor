#!/bin/bash
nohup  /program/webapp &
node_exporter  --collector.vmstat --collector.tcpstat --collector.processes
