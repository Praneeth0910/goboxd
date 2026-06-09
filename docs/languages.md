# Supported Languages

GoBoxD supports the following languages out of the box. Adding a new language requires only a YAML block in `languages.yaml` and a shell script in `scripts/lang_install/`.

---

## Language Reference

| ID | Name | Type | Source File | Build Command | Run Command |
|---|---|---|---|---|---|
| `py3` | Python 3 | Interpreted | `solution.py` | — | `/usr/bin/python3` |
| `cpp` | C++ | Compiled | `solution.cpp` | `/usr/bin/g++` | `./solution` |
| `c` | C | Compiled | `solution.c` | `/usr/bin/gcc` | `./solution` |
| `java` | Java | Compiled | `Solution.java` | `/usr/bin/javac` | `/usr/bin/java -cp . Solution` |
| `node` | Node.js (JavaScript) | Interpreted | `solution.js` | — | `/usr/bin/node` |
| `bash` | Bash | Interpreted | `solution.sh` | — | `/bin/bash` |
| `verilog` | Verilog | Compiled | `solution.v` | `/usr/bin/iverilog` | `/usr/bin/vvp` |
| `rust` | Rust | Compiled | `solution.rs` | `/usr/bin/rustc` | `./solution` |
| `go` | Go | Compiled | `solution.go` | `/usr/local/go/bin/go build` | `./solution` |
| `kotlin` | Kotlin | Compiled | `solution.kt` | `/usr/bin/kotlinc` | `/usr/bin/java -jar solution.jar` |
| `ruby` | Ruby | Interpreted | `solution.rb` | — | `/usr/bin/ruby` |
| `lua` | Lua | Interpreted | `solution.lua` | — | `/usr/bin/lua5.4` |

---

## Default Resource Limits

| ID | Wall Time (run) | Memory (run) | Max Processes |
|---|---|---|---|
| py3 | 9s | 100 MB | 100 |
| cpp | 3s | 512 MB | 64 |
| c | 3s | 512 MB | 64 |
| java | 5s | 512 MB | 100 |
| node | 9s | 100 MB | 100 |
| bash | 9s | 100 MB | 100 |
| verilog | 5s | 256 MB | 100 |
| rust | 3s | 512 MB | 64 |
| go | 3s | 512 MB | 64 |
| kotlin | 5s | 512 MB | 100 |
| ruby | 9s | 100 MB | 100 |
| lua | 9s | 100 MB | 100 |

---

## Example Snippets

### Python 3 (`py3`)
```python
n = int(input())
print(n * 2)
```

### C++ (`cpp`)
```cpp
#include <iostream>
using namespace std;
int main() {
    int n;
    cin >> n;
    cout << n * 2 << endl;
    return 0;
}
```

### C (`c`)
```c
#include <stdio.h>
int main() {
    int n;
    scanf("%d", &n);
    printf("%d\n", n * 2);
    return 0;
}
```

### Java (`java`)
```java
import java.util.Scanner;
public class Solution {
    public static void main(String[] args) {
        Scanner sc = new Scanner(System.in);
        int n = sc.nextInt();
        System.out.println(n * 2);
    }
}
```

### Node.js / JavaScript (`node`)
```javascript
const lines = [];
process.stdin.on('data', d => lines.push(...d.toString().split('\n')));
process.stdin.on('end', () => {
    const n = parseInt(lines[0]);
    console.log(n * 2);
});
```

### Bash (`bash`)
```bash
read n
echo $((n * 2))
```

### Verilog (`verilog`)
```verilog
module main;
  initial begin
    $display("hello");
    $finish;
  end
endmodule
```

### Rust (`rust`)
```rust
use std::io::{self, BufRead};
fn main() {
    let stdin = io::stdin();
    let line = stdin.lock().lines().next().unwrap().unwrap();
    let n: i64 = line.trim().parse().unwrap();
    println!("{}", n * 2);
}
```

### Go (`go`)
```go
package main

import "fmt"

func main() {
    var n int
    fmt.Scan(&n)
    fmt.Println(n * 2)
}
```

### Kotlin (`kotlin`)
```kotlin
fun main() {
    val n = readLine()!!.trim().toInt()
    println(n * 2)
}
```

### Ruby (`ruby`)
```ruby
n = gets.to_i
puts n * 2
```

### Lua (`lua`)
```lua
local n = tonumber(io.read())
print(n * 2)
```

---

## Adding a New Language

1. Create `scripts/lang_install/<id>.sh` — install the toolchain
2. Add a YAML block to `languages.yaml` under `languages:`
3. No Dockerfile change. No Go code change.

See [ADR-002](ai/adrs.md#adr-002-per-language-install-scripts-instead-of-monolithic-dockerfile-run) for the rationale.
