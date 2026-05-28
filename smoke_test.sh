#!/bin/bash
BASE="http://localhost:8080"
PASS=0
FAIL=0

check() {
    local lang=$1
    local payload=$2
    local result=$(curl -s -X POST $BASE/run \
        -H "Content-Type: application/json" \
        -d "$payload")
    local status=$(echo $result | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','?'))")
    if [ "$status" = "accepted" ]; then
        echo "✅ $lang → $status"
        PASS=$((PASS+1))
    else
        echo "❌ $lang → $status"
        echo "   $(echo $result | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('build',{}).get('stderr','') or d.get('tests',[{}])[0].get('stderr',''))" | head -c 200)"
        FAIL=$((FAIL+1))
    fi
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  goboxd language smoke test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

check "py3" '{
  "language":"py3",
  "source":"n=int(input())\nprint(n*n)",
  "tests":[{"stdin":"7","expected_stdout":"49\n"}]}'

check "cpp" '{
  "language":"cpp",
  "source":"#include<iostream>\nusing namespace std;\nint main(){int a,b;cin>>a>>b;cout<<a+b<<endl;}",
  "tests":[{"stdin":"3 5","expected_stdout":"8\n"}]}'

check "c" '{
  "language":"c",
  "source":"#include<stdio.h>\nint main(){int a,b;scanf(\"%d %d\",&a,&b);printf(\"%d\\n\",a+b);}",
  "tests":[{"stdin":"10 20","expected_stdout":"30\n"}]}'

check "java" '{
  "language":"java",
  "source_filename":"Solution.java",
  "source":"import java.util.Scanner;\npublic class Solution{\npublic static void main(String[] a){\nScanner s=new Scanner(System.in);\nSystem.out.println(s.nextInt()+s.nextInt());}}",
  "tests":[{"stdin":"4 6","expected_stdout":"10\n"}]}'

check "bash" '{
  "language":"bash",
  "source":"read a b\necho $((a+b))",
  "tests":[{"stdin":"12 8","expected_stdout":"20\n"}]}'

check "node" '{
  "language":"node",
  "source":"const lines=[];\nprocess.stdin.on(\"data\",d=>lines.push(d));\nprocess.stdin.on(\"end\",()=>{\nconst [a,b]=lines.join(\"\").trim().split(\" \").map(Number);\nconsole.log(a+b);});",
  "tests":[{"stdin":"15 25","expected_stdout":"40\n"}]}'

check "verilog" '{
  "language":"verilog",
  "source":"module main;\ninitial begin\n  $display(\"hello\");\n  $finish;\nend\nendmodule",
  "tests":[{"stdin":"","expected_stdout":"hello\n"}]}'

check "rust" '{
  "language":"rust",
  "source":"use std::io::{self,BufRead};\nfn main(){\nlet stdin=io::stdin();\nlet line=stdin.lock().lines().next().unwrap().unwrap();\nlet nums:Vec<i64>=line.split_whitespace().map(|x|x.parse().unwrap()).collect();\nprintln!(\"{}\",nums[0]+nums[1]);}",
  "tests":[{"stdin":"100 200","expected_stdout":"300\n"}]}'

check "go" '{
  "language":"go",
  "source":"package main\nimport(\"fmt\")\nfunc main(){var a,b int;fmt.Scan(&a,&b);fmt.Println(a+b)}",
  "tests":[{"stdin":"6 9","expected_stdout":"15\n"}]}'

check "kotlin" '{
  "language":"kotlin",
  "source":"fun main(){val(a,b)=readLine()!!.trim().split(\" \").map{it.toInt()};println(a+b)}",
  "tests":[{"stdin":"11 22","expected_stdout":"33\n"}]}'

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  $PASS passed  |  $FAIL failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
