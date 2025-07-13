package bsgnuplot

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	sbbs "github.com/barbell-math/smoothbrain-bs"
	sberr "github.com/barbell-math/smoothbrain-errs"
)

type (
	GnuPlot struct {
		OutFile    string
		GpltFile   *os.File
		DatFiles   []*os.File
		CsvWriters []*csv.Writer
	}

	GnuPlotOpts struct {
		GpltFile string
		DatFiles []string
		OutFile  string
		CsvSep   rune
	}
)

var (
	OpRegex = regexp.MustCompile("\\${[^{]*}")

	InvalidOpErr    = errors.New("Invalid op")
	InvalidDatOpErr = errors.New("Invalid dat op")
)

func NewGnuPlot(opts GnuPlotOpts) (GnuPlot, error) {
	gFile, err := os.Create(opts.GpltFile + ".gplt")
	if err != nil {
		return GnuPlot{}, err
	}

	datFiles := make([]*os.File, len(opts.DatFiles))
	csvWriters := make([]*csv.Writer, len(opts.DatFiles))
	for i := range len(opts.DatFiles) {
		datFiles[i], err = os.Create(opts.DatFiles[i] + ".dat")
		if err != nil {
			return GnuPlot{}, err
		}
		csvWriters[i] = csv.NewWriter(datFiles[i])
		csvWriters[i].Comma = opts.CsvSep
	}

	return GnuPlot{
		OutFile:    opts.OutFile,
		GpltFile:   gFile,
		DatFiles:   datFiles,
		CsvWriters: csvWriters,
	}, nil
}

func (g *GnuPlot) Cmds(s ...string) error {
	for _, iterS := range s {
		if resolved, err := g.getResolvedCmd(iterS); err != nil {
			return err
		} else {
			g.GpltFile.WriteString(resolved)
			g.GpltFile.WriteString("\n")
		}
	}
	return nil
}

func (g *GnuPlot) getResolvedCmd(cmd string) (string, error) {
	resolved := ""
	prevIndex := 0
	ops := OpRegex.FindAllIndex([]byte(cmd), -1)
	if len(ops) == 0 {
		return cmd, nil
	}

	for _, op := range ops {
		resolved += cmd[prevIndex:op[0]]

		subStr := cmd[op[0]+2 : op[1]-1]
		splitSubStr := strings.SplitN(subStr, ":", 2)
		switch splitSubStr[0] {
		case "dat":
			if len(splitSubStr) != 2 {
				return resolved, sberr.Wrap(
					InvalidDatOpErr,
					"Expected format: dat:<idx> Got: %s", subStr,
				)
			}
			idx, err := strconv.Atoi(splitSubStr[1])
			if err != nil {
				return resolved, sberr.AppendError(
					InvalidDatOpErr,
					sberr.InverseWrap(
						err,
						"Index was not a valid number: Expected format: dat:<idx>",
					),
				)
			}
			if idx < 0 || idx >= len(g.DatFiles) {
				return resolved, sberr.Wrap(
					InvalidDatOpErr,
					"Dat file index out of range: Got: %d Allowed Range: [0, %d)",
					idx, len(g.DatFiles),
				)
			}
			resolved += fmt.Sprintf("'%s'", g.DatFiles[idx].Name())
			prevIndex = op[1]
		case "out":
			resolved += fmt.Sprintf("'%s'", g.OutFile)
			prevIndex = op[1]
		default:
			return resolved, sberr.Wrap(InvalidOpErr, "Got: %s", splitSubStr)
		}
	}
	if len(ops[len(ops)-1]) == 2 {
		resolved += cmd[ops[len(ops)-1][1]:]
	}
	return resolved, nil
}

func (g *GnuPlot) DataRow(file int, data ...string) error {
	return g.CsvWriters[file].Write(data)
}

func (g *GnuPlot) Run(ctxt context.Context) error {
	for i := range len(g.DatFiles) {
		g.CsvWriters[i].Flush()
		g.DatFiles[i].Close()
	}
	g.GpltFile.Close()

	return sbbs.RunStdout(ctxt, "gnuplot", "-c", g.GpltFile.Name())
}
