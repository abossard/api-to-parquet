#!/bin/bash

# Set the number of commands to run
num_commands=1000

# Set the maximum number of commands to run in parallel
max_parallel=10

# Define a function to run a command in the background
run_command() {
  go run . "$1" &
}

# Loop through the number of commands to run
for ((i=1; i<=num_commands; i++)); do
  # Run the command in the background
  run_command "$TEST_URL"

  # Wait for the maximum number of commands to finish
  if ((i % max_parallel == 0)); then
    wait
  fi
done

# Wait for any remaining commands to finish
wait