package scaffolder

import _ "embed"

//go:embed assets/grex.js
var embeddedGrexJS []byte

//go:embed assets/tokens.css
var embeddedGrexTokensCSS []byte

//go:embed assets/grex.elements.json
var embeddedGrexElementsJSON []byte
