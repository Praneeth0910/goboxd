# 🚀 Beginner's Guide to goboxd Sandbox

Welcome to **goboxd**! This guide is designed for absolute beginners. You will learn how to start the sandbox, execute Python and C++ programs safely, and test its security boundaries using simple commands.

> **See also**: [Getting Started](getting-started.md) for initial setup · [Development Guide](development.md) for contributing · [API Reference](api.md) for all endpoints

---

## 🌟 What is goboxd?

**goboxd** is a secure code execution engine. It allows you to run untrusted programs (like user-submitted code in competitive programming or online IDEs) inside an isolated "jail" called a sandbox. 

The sandbox prevents malicious code from:
- Accessing your personal files.
- Modifying your operating system.
- Overusing CPU or memory resources (runaway loops).

---

## 🛠️ Prerequisites

To follow this guide, you need the following tools installed on your computer:
1. **Git**: To clone the project repository.
2. **Docker**: Runs the sandbox environment.
3. **cURL**: Sends requests to the sandbox API.
4. **jq**: (Optional but recommended) Formats and colors the sandbox outputs in your terminal.

---
# There are two ways to use this sandbox
## Method - 1: Using the API (http://localhost:8080)
## Method - 2: Using the web interface (http://localhost:8081) -> Scroll down to line 270

## Method - 1
## 📥 Step 1: Clone the Repository & Build the Image

To test `goboxd` locally, first clone the repository and build the Docker image.

### 1. Clone the project
Open your terminal and run:
```bash
git clone https://github.com/Praneeth0910/goboxd.git
cd goboxd
```

### 2. Build the Docker image
Build the sandbox image using the provided `Makefile`:
```bash
make build
```

---

## 🏃 Step 2: Start the Sandbox

Now, let's start the sandbox server using Docker. Open your terminal and run the following command:

```bash
docker run -d \
  --privileged \
  --name goboxd-service \
  -p 8080:8080 \
  goboxd:latest
```

> [!NOTE]
> The `--privileged` flag is required because the sandbox uses advanced Linux kernel features (Namespaces, cgroups, and user namespace mappings) to isolate processes.

### Verify it is Running
Run this command to check if the server is healthy:
```bash
curl -s http://localhost:8080/healthz
```
**Expected Output:**
```json
{"status":"ok"}
```

---

## 📦 How the Sandbox API Works

You communicate with the sandbox using `POST` HTTP requests sent to `http://localhost:8080/run`. 
Every request needs a JSON body containing three main pieces of information:

| Field | Description | Example |
| :--- | :--- | :--- |
| **`language`** | The language of your code (`py3` or `cpp`) | `"py3"` |
| **`source`** | Your source code as a text string | `"print('Hello World')"` |
| **`tests`** | An array of tests to run (with inputs and expected outputs) | `[{"stdin": "", "expected_stdout": ""}]` |

---

## 🐍 Step 3: Running a Python Program

Let's execute a simple Python script that calculates the area of a circle.

### 1. Create a request payload file
Save the following JSON into a file named `python_run.json`:

```json
{
  "language": "py3",
  "source": "import math\nradius = 5\narea = math.pi * (radius ** 2)\nprint(f'Radius: {radius}, Area: {area:.2f}')",
  "source_filename": "circle.py",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "Radius: 5, Area: 78.54\n"
    }
  ]
}
```

### 2. Send the code to the sandbox
Run this command in your terminal:

```bash
curl -s -X POST \
  -H "Content-Type: application/json" \
  -d @python_run.json \
  http://localhost:8080/run | jq .
```

### 3. Understand the response
You will receive a response like this:

```json
{
  "status": "accepted",
  "tests": [
    {
      "status": "accepted",
      "stdout": "Radius: 5, Area: 78.54\n",
      "stderr": "",
      "duration_ms": 52
    }
  ]
}
```

- **`status: "accepted"`** means the program executed successfully and the output matched your `expected_stdout`.
- **`duration_ms`** shows exactly how fast the execution ran (only 52 milliseconds!).

---

## 🔨 Step 4: Running a C++ Program (with Compilation)

Unlike Python, C++ must be compiled before it can run. The sandbox handles this automatically in two phases: **Build** (compilation) and **Run** (execution).

### 1. Create the request payload file
Save the following JSON into a file named `cpp_run.json`:

```json
{
  "language": "cpp",
  "source": "#include <iostream>\nint main() {\n    std::cout << \"Hello from C++!\" << std::endl;\n    return 0;\n}",
  "source_filename": "hello.cpp",
  "artifact_filename": "hello",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "Hello from C++!\n"
    }
  ]
}
```

### 2. Send the code to the sandbox
Run this command in your terminal:

```bash
curl -s -X POST \
  -H "Content-Type: application/json" \
  -d @cpp_run.json \
  http://localhost:8080/run | jq .
```

### 3. Review the compilation output
In the response, you will notice an extra `build` field:

```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 231
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "Hello from C++!\n",
      "stderr": "",
      "duration_ms": 12
    }
  ]
}
```

---

## 🛡️ Step 5: Testing Security Boundaries

The sandbox is configured to reject malicious actions automatically. Let's test two common security rules:

### Rule 1: Filename Path Traversal Prevention
If an attacker tries to read sensitive system files (e.g. `/etc/passwd`) using directory traversal tricks like `../../`, the sandbox blocks the request immediately.

**Try this command:**
```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(1)","source_filename":"../../etc/passwd","tests":[]}' | jq .
```

**Result:**
```json
{
  "error": {
    "code": "invalid_filename",
    "message": "filename contains directory traversal patterns or hidden prefixes"
  }
}
```

### Rule 2: Compiler Flag Injection Prevention
When compiling C++, you are not allowed to inject dangerous compiler options (like linking malicious external plugins or modifying runtime specs).

**Try this command:**
```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"int main(){}","build":{"flags":["-fplugin=evil.so"]},"tests":[]}' | jq .
```

**Result:**
```json
{
  "error": {
    "code": "disallowed_flag",
    "message": "compiler flag \"-fplugin=evil.so\" is not allowed"
  }
}
```

---

## 🧹 Step 6: Clean Up

When you are done experimenting, you can stop and remove the sandbox container with:

```bash
docker stop goboxd-service && docker rm goboxd-service
```

---

## ❓ Troubleshooting

> [!WARNING]
> **Error: `docker: Error response from daemon: Port 8080 is already allocated.`**
> This means another service on your host machine is already using port 8080. 
> To resolve this, map the container to a different local port (e.g., `8085`):
> ```bash
> docker run -d --privileged --name goboxd-service -p 8085:8080 goboxd:latest
> ```
> Then, make your curl requests to `http://localhost:8085/run`.

---
## Method 2 : Using the web interface (http://localhost:8081)

The `goboxd` repository includes a fully-functional, single-page web UI that lets you write, test, and execute code interactively against your running sandbox API. 

The demo interface provides:
- Live language loading from the API
- A premium Monaco Editor with syntax highlighting and a custom dark theme
- Multi-test case support with expected output matching
- Live build logs and per-test execution details

### Step 1: Start the `goboxd` Server
Ensure the backend API is running on port 8080 (the default port that the demo expects):
```bash
make build && make run
```
*(Alternatively, run the container via `docker-compose up -d` or `docker run` as shown in previous sections).*

### Step 2: Serve the Demo UI
Open a **new terminal** window, navigate to the project directory, and start a simple web server to serve the `index.html` file:
```bash
cd docs/demo
python3 -m http.server 8081
```

### Step 3: Open the UI in Your Browser
Navigate to [http://localhost:8081](http://localhost:8081) in your web browser. 

- You should see the language dropdown automatically populate with the available languages.
- You can write your code, define test cases with `stdin` and `expected stdout`, and click the **Run** button (or press `Ctrl+Enter`) to test your code in real-time.
