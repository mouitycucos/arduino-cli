// This file is part of arduino-cli.
//
// Copyright 2020 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	bldr "github.com/arduino/arduino-cli/arduino/builder"
	"github.com/arduino/arduino-cli/legacy/builder/types"
	"github.com/arduino/arduino-cli/legacy/builder/utils"
	properties "github.com/arduino/go-properties-orderedmap"
	"github.com/pkg/errors"
)

func PreprocessSketchWithArduinoPreprocessor(ctx *types.Context) error {
	if err := ctx.PreprocPath.MkdirAll(); err != nil {
		return errors.WithStack(err)
	}

	sourceFile := ctx.SketchBuildPath.Join(ctx.Sketch.MainFile.Base() + ".cpp")
	GCCPreprocRunner(ctx, sourceFile, ctx.PreprocPath.Join("ctags_target_for_gcc_minus_e.cpp"), ctx.IncludeFolders)

	targetFilePath := ctx.PreprocPath.Join("ctags_target_for_gcc_minus_e.cpp")
	buildProperties := properties.NewMap()
	buildProperties.Set("tools.arduino-preprocessor.path", "{runtime.tools.arduino-preprocessor.path}")
	buildProperties.Set("tools.arduino-preprocessor.cmd.path", "{path}/arduino-preprocessor")
	buildProperties.Set("tools.arduino-preprocessor.pattern", `"{cmd.path}" "{source_file}" -- -std=gnu++11`)
	buildProperties.Set("preproc.macros.flags", "-w -x c++ -E -CC")
	buildProperties.Merge(ctx.BuildProperties)
	buildProperties.Merge(buildProperties.SubTree("tools").SubTree("arduino-preprocessor"))
	buildProperties.SetPath("source_file", targetFilePath)

	pattern := buildProperties.Get("pattern")
	if pattern == "" {
		return errors.New(tr("arduino-preprocessor pattern is missing"))
	}

	commandLine := buildProperties.ExpandPropsInString(pattern)
	parts, err := properties.SplitQuotedString(commandLine, `"'`, false)
	if err != nil {
		return errors.WithStack(err)
	}
	command := exec.Command(parts[0], parts[1:]...)
	command.Env = append(os.Environ(), ctx.PackageManager.GetEnvVarsForSpawnedProcess()...)

	if runtime.GOOS == "windows" {
		// chdir in the uppermost directory to avoid UTF-8 bug in clang (https://github.com/arduino/arduino-preprocessor/issues/2)
		command.Dir = filepath.VolumeName(command.Args[0]) + "/"
		//command.Args[0], _ = filepath.Rel(command.Dir, command.Args[0])
	}

	verbose := ctx.Verbose
	if verbose {
		fmt.Println(commandLine)
	}

	buf, err := command.Output()
	if err != nil {
		return errors.New(errors.WithStack(err).Error() + string(err.(*exec.ExitError).Stderr))
	}

	result := utils.NormalizeUTF8(buf)

	//fmt.Printf("PREPROCESSOR OUTPUT:\n%s\n", output)
	ctx.Source = string(result)

	return bldr.SketchSaveItemCpp(ctx.Sketch.MainFile, []byte(ctx.Source), ctx.SketchBuildPath)
}
