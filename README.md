# Redis Queue Bridge

## Overview
This Go application serves as a bridge for transferring data between two Redis queues. It is designed to handle robust, concurrent data transfers with comprehensive error handling and retry mechanisms.

## Features
- **Concurrency Support**: Utilizes multiple workers to parallelize the transfer of data.
- **Error Handling**: Implements retries for failed push operations and exits on persistent failures.
- **Signal Handling**: Gracefully handles termination signals to ensure clean shutdowns.
- **Logging**: Integrates with syslog for logging operational data.
- **Dynamic Configuration**: Supports command-line arguments to configure source and destination Redis servers, keys, and other operational parameters.

## Requirements
- Go (version 1.15 or higher recommended)
- Redis server instances accessible from the host running this application
- A Unix-like operating system for signal handling and syslog integration

## Installation
1. **Clone the repository:**
   ```bash
   git clone https://github.com/CK200/golang-redis-bridge.git
   cd golang-redis-bridge
   ```

2. **Build the application:**
   ```bash
   go build -o redis-queue-bridge
   ```

## Usage
Run the application with the necessary parameters:

bash
```bash
./redis-queue-bridge -s [source-redis-server] -d [destination-redis-server] -k [source-key] -t [destination-key] -c [concurrency-level] -n [process-name]
```

### Parameters:
- `-s, --src`: Source Redis server address (default: `127.0.0.1:6379`)
- `-d, --dest`: Destination Redis server address (default: `127.0.0.1:6379`)
- `-k, --srckey`: Redis key for the source queue
- `-t, --destkey`: Redis key for the destination queue
- `-c, --concurrency`: Number of concurrent workers (default: `10`)
- `-n, --pname`: Process name for syslog logging (default: `goRedisBridge`)

### Example
```bash
./redis-queue-bridge -s localhost:6379 -d localhost:6380 -k queue_src -t queue_dest -c 5 -n RedisBridge
```

## Logging
Logs are sent to syslog with the specified process name. Ensure your syslog daemon is configured to capture and store these logs as needed.

## Contributing
Contributions to this project are welcome. Please fork the repository, make your changes, and submit a pull request.

## License
MIT License

Copyright (c) 2025 CK

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
