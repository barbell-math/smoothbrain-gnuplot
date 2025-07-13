// A very simple library that aids in creating plots with gnuplot.
package sbgnuplot

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	sberr "github.com/barbell-math/smoothbrain-errs"
)

type (
	// The main struct that is used to control plot generation.
	GnuPlot struct {
		outFile    string
		gpltFile   *os.File
		datFiles   []*os.File
		csvWriters []*csv.Writer
	}

	GnuPlotOpts struct {
		// Specifies the file where the generated gnuplot code will go. This
		// path will be relative to the current directory.
		GpltFile string
		// Specifies the files where the data for the plot will be written to.
		// The order of the files matters because methods on [GnuPlot] will
		// reference a data file by index.
		// All paths will be relative to the current directory.
		DatFiles []string
		// Specifies the file where the generated plot will be written to. This
		// path will be relative to the current directory.
		OutFile string
		// The column delimiter character that should be used when writing the
		// data to the dat files.
		CsvSep rune
	}
)

var (
	// The regex that matches strings that need to be replaced in the supplied
	// cmds. The exact contents of the string found by the regular expression
	// will determine what it is replaced with.
	OpRegex = regexp.MustCompile("\\${[^{]*}")

	InvalidOpErr       = errors.New("Invalid op")
	InvalidDatOpErr    = errors.New("Invalid dat op")
	InvalidDatIndexErr = errors.New("Invalid data index")
)

// Creates a new [GnuPlot] struct with the supplied options. All data and gnu
// plot code files will be created. The output file will be created by gnu plot
// itself when the [GnuPlot.Run] method is called.
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
		outFile:    opts.OutFile,
		gpltFile:   gFile,
		datFiles:   datFiles,
		csvWriters: csvWriters,
	}, nil
}

// Writes cmds to the gnu plot code file. The cmds will be parsed for
// operations. An operation will replace the given text with a specific value.
// Valid operations are as follows:
//
//   - {out}: Replaces `{out}` with the path of the out file
//   - {dat:#}: Replaces `{dat:#}` with the path of the data file at the index
//     specified by `#`. If `#` is not a valid number, a negative number, or
//     a number outside the range of the data file list an error will be
//     returned and none of the supplied cmds will be added
func (g *GnuPlot) Cmds(s ...string) error {
	for _, iterS := range s {
		if resolved, err := g.getResolvedCmd(iterS); err != nil {
			return err
		} else {
			g.gpltFile.WriteString(resolved)
			g.gpltFile.WriteString("\n")
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
			if idx < 0 || idx >= len(g.datFiles) {
				return resolved, sberr.Wrap(
					InvalidDatIndexErr,
					"Dat file index out of range: Got: %d Allowed Range: [0, %d)",
					idx, len(g.datFiles),
				)
			}
			resolved += fmt.Sprintf("'%s'", g.datFiles[idx].Name())
			prevIndex = op[1]
		case "out":
			resolved += fmt.Sprintf("'%s'", g.outFile)
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

// Writes a data row to the data file specified by the `file` index. If the
// index specified by `file` is invalid a [InvalidDatIndexErr] will be returned.
//
// To write an empty line call this method with a single empty string as the
// data arguments.
//
// If no data arguments are provided no work will be done and no error will be
// returned.
func (g *GnuPlot) DataRow(file int, data ...string) error {
	if len(data) <= 0 {
		return nil
	}
	if file < 0 || file >= len(g.csvWriters) {
		return sberr.Wrap(
			InvalidDatIndexErr,
			"Dat file index out of range: Got: %d Allowed Range: [0, %d)",
			file, len(g.csvWriters),
		)
	}
	return g.csvWriters[file].Write(data)
}

// Flushes all writers and executes gnuplot with the generated gnu plot code and
// data files. All open files are closed so the gnuplot object should not be
// used after calling this method.
func (g *GnuPlot) Run(ctxt context.Context) error {
	for i := range len(g.datFiles) {
		g.csvWriters[i].Flush()
		g.datFiles[i].Close()
	}
	g.gpltFile.Close()

	var cmd *exec.Cmd
	cmd = exec.CommandContext(ctxt, "gnuplot", "-c", g.gpltFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
