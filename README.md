# calcu

A unit of measure aware & yacc-based calculator/interpreter designed for generically calculating greenhouse gas
emissions.

## How to use

```go
package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/maxnilz/calcu"
)

func main() {
	exprs := `
CO2 = activity_value * CO2Factor;
CH2 = activity_value * CH2Factor;
N2O = activity_value * N2OFactor;
GHG = CO2 + CH2 + N2O;
a = CO2 * CH2 * (1 + 2);
b = CO2 * CH2 * (1 - 2);
c = CO2 * CH2 * 2/1;
d = CO2 * CH2 * (2/1);
print(CO2, CH2, N2O, GHG, a, b, c, d);
print(CO2, CH2, N2O, GHG);
`
	vars := map[string]string{
		"activity_value": "1(10^3m3)",
		"CO2Factor":      "1.1E-04Gg/10^3m3",
		"CH2Factor":      "7.2E-06Gg/10^3m3",
		"N2OFactor":      "1.1E-03Gg/10^3m3",
	}
	intrp, err := calcu.NewInterpreter(vars)
	if err != nil {
		log.Fatal(err)
	}
	rd := bytes.NewBufferString(exprs)
	outvars, err := intrp.Interpret(rd)
	if err != nil {
		log.Fatal(err)
	}
	co2 := outvars["CO2"]
	ch2 := outvars["CH2"]
	n2o := outvars["N2O"]
	ghg := outvars["GHG"]
	a := outvars["a"]
	b := outvars["b"]
	c := outvars["c"]
	d := outvars["d"]

	fmt.Println(co2, ch2, n2o, ghg, a, b, c, d)
}
```

## Build

1. download goyacc
    ```bash
    $ go install golang.org/x/tools/cmd/goyacc@latest
    ```
2. generate yacc parser

    ```
   $ go generate ./...
    ```

