# JSON Payloads

This document contains the JSON payload requests and their corresponding output responses for the Scala, Swift, and TypeScript test cases.

## Scala

### scala/accepted.json

**Request Payload:**
```json
{
  "language": "scala",
  "source": "object Main {\n    def main(args: Array[String]): Unit = {\n        val n = scala.io.StdIn.readInt()\n        println(n * 2)\n    }\n}",
  "source_filename": "Main.scala",
  "artifact_filename": "Main",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "accepted",
    "build": {
        "status": "ok",
        "stdout": "[0.002s][warning][os,container] Cgroup memory controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2093', detected limits won't be accurate\n[0.003s][warning][os,container] Cgroup cpu controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2093', detected limits won't be accurate\n",
        "stderr": "",
        "duration_ms": 7531
    },
    "tests": [
        {
            "status": "accepted",
            "stdout": "42\n",
            "stderr": "/usr/bin/scala: line 50: /dev/null: No such file or directory\n",
            "duration_ms": 1373,
            "memory_peak_kb": 0
        }
    ]
}
```

### scala/build_failed.json

**Request Payload:**
```json
{
  "language": "scala",
  "source": "object Main {\n    def main(args: Array[String]): Unit = {\n        println(\"x\")",
  "source_filename": "Main.scala",
  "artifact_filename": "Main",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "build_failed",
    "build": {
        "status": "failed",
        "stdout": "[0.001s][warning][os,container] Cgroup memory controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2282', detected limits won't be accurate\n[0.002s][warning][os,container] Cgroup cpu controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2282', detected limits won't be accurate\n",
        "stderr": "/tmp/goboxd-1-22-9017933166132432/solution.scala:3: error: '}' expected but eof found.\n        println(\"x\")\n                    ^\none error found\n",
        "duration_ms": 4377
    },
    "tests": [
        {
            "status": "not_executed",
            "stdout": "",
            "stderr": "",
            "duration_ms": 0,
            "memory_peak_kb": 0
        }
    ]
}
```

### scala/runtime_error.json

**Request Payload:**
```json
{
  "language": "scala",
  "source": "object Main {\n    def main(args: Array[String]): Unit = throw new RuntimeException(\"boom\")\n}",
  "source_filename": "Main.scala",
  "artifact_filename": "Main",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "runtime_error",
    "build": {
        "status": "ok",
        "stdout": "[0.001s][warning][os,container] Cgroup memory controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2318', detected limits won't be accurate\n[0.001s][warning][os,container] Cgroup cpu controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2318', detected limits won't be accurate\n",
        "stderr": "",
        "duration_ms": 6120
    },
    "tests": [
        {
            "status": "runtime_error",
            "stdout": "",
            "stderr": "/usr/bin/scala: line 50: /dev/null: No such file or directory\njava.lang.RuntimeException: boom\n\tat Main$.main(solution.scala:2)\n\tat Main.main(solution.scala)\n\tat java.base/jdk.internal.reflect.DirectMethodHandleAccessor.invoke(DirectMethodHandleAccessor.java:103)\n\tat java.base/java.lang.reflect.Method.invoke(Method.java:580)\n\tat scala.reflect.internal.util.ScalaClassLoader$$anonfun$run$1.apply(ScalaClassLoader.scala:70)\n\tat scala.reflect.internal.util.ScalaClassLoader$class.asContext(ScalaClassLoader.scala:31)\n\tat scala.reflect.internal.util.ScalaClassLoader$URLClassLoader.asContext(ScalaClassLoader.scala:101)\n\tat scala.reflect.internal.util.ScalaClassLoader$class.run(ScalaClassLoader.scala:70)\n\tat scala.reflect.internal.util.ScalaClassLoader$URLClassLoader.run(ScalaClassLoader.scala:101)\n\tat scala.tools.nsc.CommonRunner$class.run(ObjectRunner.scala:22)\n\tat scala.tools.nsc.JarRunner$.run(MainGenericRunner.scala:13)\n\tat scala.tools.nsc.CommonRunner$class.runAndCatch(ObjectRunner.scala:29)\n\tat scala.tools.nsc.JarRunner$.runJar(MainGenericRunner.scala:25)\n\tat scala.tools.nsc.MainGenericRunner.runTarget$1(MainGenericRunner.scala:69)\n\tat scala.tools.nsc.MainGenericRunner.run$1(MainGenericRunner.scala:87)\n\tat scala.tools.nsc.MainGenericRunner.process(MainGenericRunner.scala:98)\n\tat scala.tools.nsc.MainGenericRunner$.main(MainGenericRunner.scala:103)\n\tat scala.tools.nsc.MainGenericRunner.main(MainGenericRunner.scala)\n",
            "duration_ms": 1025,
            "memory_peak_kb": 0
        }
    ]
}
```

### scala/time_exceeded.json

**Request Payload:**
```json
{
  "language": "scala",
  "source": "object Main {\n    def main(args: Array[String]): Unit = while (true) {}\n}",
  "source_filename": "Main.scala",
  "artifact_filename": "Main",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ],
  "run": {
    "limits": {
      "wall_time_s": 1
    }
  }
}
```

**Response Output:**
```json
{
    "status": "time_exceeded",
    "build": {
        "status": "ok",
        "stdout": "[0.001s][warning][os,container] Cgroup memory controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2390', detected limits won't be accurate\n[0.001s][warning][os,container] Cgroup cpu controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2390', detected limits won't be accurate\n",
        "stderr": "",
        "duration_ms": 6065
    },
    "tests": [
        {
            "status": "time_exceeded",
            "stdout": "",
            "stderr": "/usr/bin/scala: line 50: /dev/null: No such file or directory\n",
            "duration_ms": 1026,
            "memory_peak_kb": 0
        }
    ]
}
```

### scala/wrong_output.json

**Request Payload:**
```json
{
  "language": "scala",
  "source": "object Main {\n    def main(args: Array[String]): Unit = {\n        val n = scala.io.StdIn.readInt()\n        println(n * 2)\n    }\n}",
  "source_filename": "Main.scala",
  "artifact_filename": "Main",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "43\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "wrong_output",
    "build": {
        "status": "ok",
        "stdout": "[0.002s][warning][os,container] Cgroup memory controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2462', detected limits won't be accurate\n[0.003s][warning][os,container] Cgroup cpu controller path at '/sys/fs/cgroup' seems to have moved to '/../../NSJAIL.2462', detected limits won't be accurate\n",
        "stderr": "",
        "duration_ms": 5919
    },
    "tests": [
        {
            "status": "wrong_output",
            "stdout": "42\n",
            "stderr": "/usr/bin/scala: line 50: /dev/null: No such file or directory\n",
            "duration_ms": 1180,
            "memory_peak_kb": 0
        }
    ]
}
```

## Swift

### swift/accepted.json

**Request Payload:**
```json
{
  "language": "swift",
  "source": "import Foundation\nlet n = Int(readLine()!.trimmingCharacters(in: .whitespaces))!\nprint(n * 2)",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "accepted",
    "build": {
        "status": "ok",
        "stdout": "",
        "stderr": "",
        "duration_ms": 3476
    },
    "tests": [
        {
            "status": "accepted",
            "stdout": "42\n",
            "stderr": "",
            "duration_ms": 35,
            "memory_peak_kb": 0
        }
    ]
}
```

### swift/build_failed.json

**Request Payload:**
```json
{
  "language": "swift",
  "source": "let n = ",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "build_failed",
    "build": {
        "status": "failed",
        "stdout": "error: fatalError\n",
        "stderr": "/tmp/goboxd-1-27-c2dace4089034469/solution.swift:1:9: error: expected initial value after '='\nlet n = \n        ^\n",
        "duration_ms": 266
    },
    "tests": [
        {
            "status": "not_executed",
            "stdout": "",
            "stderr": "",
            "duration_ms": 0,
            "memory_peak_kb": 0
        }
    ]
}
```

### swift/runtime_error.json

**Request Payload:**
```json
{
  "language": "swift",
  "source": "fatalError(\"boom\")",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "runtime_error",
    "build": {
        "status": "ok",
        "stdout": "",
        "stderr": "",
        "duration_ms": 720
    },
    "tests": [
        {
            "status": "runtime_error",
            "stdout": "",
            "stderr": "solution/solution.swift:1: Fatal error: boom\n",
            "duration_ms": 39,
            "memory_peak_kb": 0
        }
    ]
}
```

### swift/time_exceeded.json

**Request Payload:**
```json
{
  "language": "swift",
  "source": "while true {}",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ],
  "run": {
    "limits": {
      "wall_time_s": 1
    }
  }
}
```

**Response Output:**
```json
{
    "status": "time_exceeded",
    "build": {
        "status": "ok",
        "stdout": "",
        "stderr": "",
        "duration_ms": 768
    },
    "tests": [
        {
            "status": "time_exceeded",
            "stdout": "",
            "stderr": "",
            "duration_ms": 1009,
            "memory_peak_kb": 0
        }
    ]
}
```

### swift/wrong_output.json

**Request Payload:**
```json
{
  "language": "swift",
  "source": "import Foundation\nlet n = Int(readLine()!.trimmingCharacters(in: .whitespaces))!\nprint(n * 2)",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "43\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "wrong_output",
    "build": {
        "status": "ok",
        "stdout": "",
        "stderr": "",
        "duration_ms": 2633
    },
    "tests": [
        {
            "status": "wrong_output",
            "stdout": "42\n",
            "stderr": "",
            "duration_ms": 37,
            "memory_peak_kb": 0
        }
    ]
}
```

## Typescript

### typescript/accepted.json

**Request Payload:**
```json
{
  "language": "typescript",
  "source": "declare const process: any;\nlet data = '';\nprocess.stdin.on('data', (chunk: string) => { data += chunk; });\nprocess.stdin.on('end', () => {\n  const n: number = parseInt(data.trim(), 10);\n  console.log(n * 2);\n});",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "accepted",
    "build": {
        "status": "ok",
        "stdout": "",
        "stderr": "",
        "duration_ms": 5805
    },
    "tests": [
        {
            "status": "accepted",
            "stdout": "42\n",
            "stderr": "Warning: disabling flag --expose_wasm due to conflicting flags\n",
            "duration_ms": 201,
            "memory_peak_kb": 0
        }
    ]
}
```

### typescript/build_failed.json

**Request Payload:**
```json
{
  "language": "typescript",
  "source": "const n number = 5;",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "build_failed",
    "build": {
        "status": "failed",
        "stdout": "solution.ts(1,9): error TS1005: ',' expected.\n",
        "stderr": "",
        "duration_ms": 6378
    },
    "tests": [
        {
            "status": "not_executed",
            "stdout": "",
            "stderr": "",
            "duration_ms": 0,
            "memory_peak_kb": 0
        }
    ]
}
```

### typescript/runtime_error.json

**Request Payload:**
```json
{
  "language": "typescript",
  "source": "throw new Error('boom');",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "runtime_error",
    "build": {
        "status": "ok",
        "stdout": "",
        "stderr": "",
        "duration_ms": 6373
    },
    "tests": [
        {
            "status": "runtime_error",
            "stdout": "",
            "stderr": "Warning: disabling flag --expose_wasm due to conflicting flags\n/solution.js:2\nthrow new Error('boom');\n^\n\nError: boom\n    at Object.<anonymous> (/solution.js:2:7)\n    at Module._compile (node:internal/modules/cjs/loader:1529:14)\n    at Module._extensions..js (node:internal/modules/cjs/loader:1613:10)\n    at Module.load (node:internal/modules/cjs/loader:1275:32)\n    at Module._load (node:internal/modules/cjs/loader:1096:12)\n    at Function.executeUserEntryPoint [as runMain] (node:internal/modules/run_main:164:12)\n    at node:internal/main/run_main_module:28:49\n\nNode.js v20.19.2\n",
            "duration_ms": 226,
            "memory_peak_kb": 0
        }
    ]
}
```

### typescript/time_exceeded.json

**Request Payload:**
```json
{
  "language": "typescript",
  "source": "let i: number = 0;\nwhile (true) { i += 1; }",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "time_exceeded",
    "build": {
        "status": "ok",
        "stdout": "",
        "stderr": "",
        "duration_ms": 6056
    },
    "tests": [
        {
            "status": "time_exceeded",
            "stdout": "",
            "stderr": "Warning: disabling flag --expose_wasm due to conflicting flags\n",
            "duration_ms": 9015,
            "memory_peak_kb": 0
        }
    ]
}
```

### typescript/wrong_output.json

**Request Payload:**
```json
{
  "language": "typescript",
  "source": "declare const process: any;\nlet data = '';\nprocess.stdin.on('data', (chunk: string) => { data += chunk; });\nprocess.stdin.on('end', () => {\n  const n: number = parseInt(data.trim(), 10);\n  console.log(n * 2);\n});",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "43\n"
    }
  ]
}
```

**Response Output:**
```json
{
    "status": "wrong_output",
    "build": {
        "status": "ok",
        "stdout": "",
        "stderr": "",
        "duration_ms": 5555
    },
    "tests": [
        {
            "status": "wrong_output",
            "stdout": "42\n",
            "stderr": "Warning: disabling flag --expose_wasm due to conflicting flags\n",
            "duration_ms": 230,
            "memory_peak_kb": 0
        }
    ]
}
```

