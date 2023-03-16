#!/bin/bash

if [ ! -f "/app/act_runner/act_runner/.runner" ]; then
  /app/act_runner/act_runner register --instance ${INSTANCE} --token ${TOKEN} --no-interactive
fi

/app/act_runner/act_runner daemon
