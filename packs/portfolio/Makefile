# packs/portfolio — thin wrappers over the CLI, matching the other packs.
.PHONY: help validate report text import share test

help:  ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n",$$1,$$2}'

validate:  ## check every data file against its schema
	@python3 main.py validate

report:  ## regenerate the explorer page and data document
	@python3 main.py report

text:  ## print the sell ladder to stdout
	@python3 main.py report --format text

import:  ## import a broker export (FILE=... PROFILE=schwab, add DRY=1)
	@python3 main.py import --file "$(FILE)" --profile "$(PROFILE)" $(if $(DRY),--dry-run,)

share:  ## produce a redacted copy (PROFILE=figures-hidden)
	@python3 main.py share --profile "$(PROFILE)"

test:  ## run the test suite (parity is the gate)
	@python3 -m unittest discover -s tests -t . -v
