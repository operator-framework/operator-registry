
## Veneers 
File-Based Catalogs (FBC) are a major improvement to the imperative update graph approaches of previous versions.  FBCs give operator authors a [declarative and deterministic approach to defining their update graph](https://olm.operatorframework.io/docs/concepts/olm-architecture/operator-catalog/creating-an-update-graph/).  Unfortunately, FBCs can get complex, especially as the number of releases and dependencies scale.

Enter the `veneer`, a simpler transitional step to generating an FBC.  The term is a bit of a misnomer, because a `veneer` isn’t pretending to be something else.

There are two components to every `veneer`:
1. An arbitrary API
2. An executable which processes #1 and produces a valid FBC.

`veneer` benefits:
- Consistent expectations by including examples with `opm`
- A good set of use-cases for simplifying the ability of operator authors to manage FBCs, including contributing them to an index


```
.
├── basic
└── semver
```

These are not intended to cover all cases, and we welcome contribution, forking, and discussion. 
