package prune

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/vercel/turborepo/cli/cmdutil"
	"github.com/vercel/turborepo/cli/internal/cache"
	"github.com/vercel/turborepo/cli/internal/context"
	"github.com/vercel/turborepo/cli/internal/fs"
	"github.com/vercel/turborepo/cli/internal/turbopath"
	"github.com/vercel/turborepo/cli/internal/ui"

	"github.com/fatih/color"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type opts struct {
	scope     string
	docker    bool
	outputDir string
}

func addPruneFlags(opts *opts, flags *pflag.FlagSet) {
	flags.StringVar(&opts.scope, "scope", "", "Specify package to act as entry point for pruned monorepo (required).")
	flags.BoolVar(&opts.docker, "docker", false, "Output pruned workspace into 'full' and 'json' directories optimized for Docker layer caching.")
	flags.StringVar(&opts.outputDir, "out-dir", "out", "Set the root directory for files output by this command")
	// No-op the cwd flag while the root level command is not yet cobra
	_ = flags.String("cwd", "", "")
	if err := flags.MarkHidden("cwd"); err != nil {
		// Fail fast if we have misconfigured our flags
		panic(err)
	}
}

func GetCmd(helper *cmdutil.Helper) *cobra.Command {
	opts := &opts{}
	cmd := &cobra.Command{
		Use:                   "prune --scope=<package name> [<flags>]",
		Short:                 "Prepare a subset of your monorepo.",
		SilenceUsage:          true,
		SilenceErrors:         true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := helper.GetCmdBase(cmd.Flags())
			if err != nil {
				return err
			}
			if opts.scope == "" {
				return base.LogError("at least one target must be specified")
			}
			p := &prune{
				base,
			}
			if err := p.prune(opts); err != nil {
				logError(p.base.Logger, p.base.UI, err)
				return err
			}
			return nil
		},
	}
	addPruneFlags(opts, cmd.Flags())
	return cmd
}

func logError(logger hclog.Logger, ui cli.Ui, err error) {
	logger.Error("error", err)
	pref := color.New(color.Bold, color.FgRed, color.ReverseVideo).Sprint(" ERROR ")
	ui.Error(fmt.Sprintf("%s%s", pref, color.RedString(" %v", err)))
}

type prune struct {
	base *cmdutil.CmdBase
}

// Prune creates a smaller monorepo with only the required workspaces
func (p *prune) prune(opts *opts) error {
	cacheDir := cache.DefaultLocation(p.base.RepoRoot)
	rootPackageJSONPath := p.base.RepoRoot.Join("package.json")
	rootPackageJSON, err := fs.ReadPackageJSON(rootPackageJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}
	ctx, err := context.New(context.WithGraph(p.base.RepoRoot, rootPackageJSON, cacheDir))
	if err != nil {
		return errors.Wrap(err, "could not construct graph")
	}
	p.base.Logger.Trace("scope", "value", opts.scope)
	target, scopeIsValid := ctx.PackageInfos[opts.scope]
	if !scopeIsValid {
		return errors.Errorf("invalid scope: package %v not found", opts.scope)
	}
	outDir := p.base.RepoRoot.Join(opts.outputDir)
	fullDir := outDir
	if opts.docker {
		fullDir = fullDir.Join("full")
	}

	p.base.Logger.Trace("target", "value", target.Name)
	p.base.Logger.Trace("directory", "value", target.Dir)
	p.base.Logger.Trace("external deps", "value", target.UnresolvedExternalDeps)
	p.base.Logger.Trace("internal deps", "value", target.InternalDeps)
	p.base.Logger.Trace("docker", "value", opts.docker)
	p.base.Logger.Trace("out dir", "value", outDir.ToString())

	canPrune, err := ctx.PackageManager.CanPrune(p.base.RepoRoot)
	if err != nil {
		return err
	}
	if !canPrune {
		return errors.Errorf("this command is not yet implemented for %s", ctx.PackageManager.Name)
	}

	p.base.UI.Output(fmt.Sprintf("Generating pruned monorepo for %v in %v", ui.Bold(opts.scope), ui.Bold(outDir.ToString())))

	packageJSONPath := outDir.Join("package.json")
	if err := packageJSONPath.EnsureDir(); err != nil {
		return errors.Wrap(err, "could not create output directory")
	}
	workspaces := []turbopath.AnchoredSystemPath{}
	lockfile := rootPackageJSON.SubLockfile
	targets := []interface{}{opts.scope}
	internalDeps, err := ctx.TopologicalGraph.Ancestors(opts.scope)
	if err != nil {
		return errors.Wrap(err, "could find traverse the dependency graph to find topological dependencies")
	}
	targets = append(targets, internalDeps.List()...)

	for _, internalDep := range targets {
		if internalDep == ctx.RootNode {
			continue
		}
		workspaces = append(workspaces, ctx.PackageInfos[internalDep].Dir)
		targetDir := fullDir.Join(ctx.PackageInfos[internalDep].Dir.ToStringDuringMigration())
		if err := targetDir.EnsureDir(); err != nil {
			return errors.Wrapf(err, "failed to create folder %v for %v", targetDir, internalDep)
		}
		if err := fs.RecursiveCopy(ctx.PackageInfos[internalDep].Dir.ToStringDuringMigration(), targetDir.ToStringDuringMigration()); err != nil {
			return errors.Wrapf(err, "failed to copy %v into %v", internalDep, targetDir)
		}
		if opts.docker {
			jsonDir := outDir.Join("json", ctx.PackageInfos[internalDep].PackageJSONPath.ToStringDuringMigration())
			if err := jsonDir.EnsureDir(); err != nil {
				return errors.Wrapf(err, "failed to create folder %v for %v", jsonDir, internalDep)
			}
			if err := fs.RecursiveCopy(ctx.PackageInfos[internalDep].PackageJSONPath.ToStringDuringMigration(), jsonDir.ToStringDuringMigration()); err != nil {
				return errors.Wrapf(err, "failed to copy %v into %v", internalDep, jsonDir)
			}
		}

		for k, v := range ctx.PackageInfos[internalDep].SubLockfile {
			lockfile[k] = v
		}

		p.base.UI.Output(fmt.Sprintf(" - Added %v", ctx.PackageInfos[internalDep].Name))
	}
	p.base.Logger.Trace("new workspaces", "value", workspaces)
	if fs.FileExists(".gitignore") {
		if err := fs.CopyFile(&fs.LstatCachedFile{Path: p.base.RepoRoot.Join(".gitignore")}, fullDir.Join(".gitignore").ToStringDuringMigration()); err != nil {
			return errors.Wrap(err, "failed to copy root .gitignore")
		}
	}

	if fs.FileExists("turbo.json") {
		if err := fs.CopyFile(&fs.LstatCachedFile{Path: p.base.RepoRoot.Join("turbo.json")}, fullDir.Join("turbo.json").ToStringDuringMigration()); err != nil {
			return errors.Wrap(err, "failed to copy root turbo.json")
		}
	}

	if err := fs.CopyFile(&fs.LstatCachedFile{Path: p.base.RepoRoot.Join("package.json")}, fullDir.Join("package.json").ToStringDuringMigration()); err != nil {
		return errors.Wrap(err, "failed to copy root package.json")
	}

	if opts.docker {
		if err := fs.CopyFile(&fs.LstatCachedFile{Path: p.base.RepoRoot.Join("package.json")}, outDir.Join("json", "package.json").ToStringDuringMigration()); err != nil {
			return errors.Wrap(err, "failed to copy root package.json")
		}
	}

	var b bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&b)
	yamlEncoder.SetIndent(2)
	if err := yamlEncoder.Encode(lockfile); err != nil {
		return errors.Wrap(err, "failed to materialize sub-lockfile. This can happen if your lockfile contains merge conflicts or is somehow corrupted. Please report this if it occurs")
	}
	if err := outDir.Join("yarn.lock").WriteFile(b.Bytes(), fs.DirPermissions); err != nil {
		return errors.Wrap(err, "failed to write sub-lockfile")
	}

	yarnTmpFilePath := outDir.Join("yarn-tmp.lock")
	tmpGeneratedLockfile, err := yarnTmpFilePath.Create()
	if err != nil {
		return errors.Wrap(err, "failed create temporary lockfile")
	}
	tmpGeneratedLockfileWriter := bufio.NewWriter(tmpGeneratedLockfile)

	if ctx.PackageManager.Name == "nodejs-yarn" {
		tmpGeneratedLockfileWriter.WriteString("# THIS IS AN AUTOGENERATED FILE. DO NOT EDIT THIS FILE DIRECTLY.\n# yarn lockfile v1\n\n")
	} else {
		tmpGeneratedLockfileWriter.WriteString("# This file is generated by running \"yarn install\" inside your project.\n# Manual changes might be lost - proceed with caution!\n\n__metadata:\n  version: 5\n  cacheKey: 8\n\n")
	}

	// because of yarn being yarn, we need to inject lines in between each block of YAML to make it "valid" SYML
	lockFilePath := outDir.Join("yarn.lock")
	generatedLockfile, err := lockFilePath.Open()
	if err != nil {
		return errors.Wrap(err, "failed to massage lockfile")
	}

	scan := bufio.NewScanner(generatedLockfile)
	buf := make([]byte, 0, 1024*1024)
	scan.Buffer(buf, 10*1024*1024)
	for scan.Scan() {
		line := scan.Text() //Writing to Stdout
		if !strings.HasPrefix(line, " ") {
			tmpGeneratedLockfileWriter.WriteString(fmt.Sprintf("\n%v\n", strings.ReplaceAll(line, "'", "\"")))
		} else {
			tmpGeneratedLockfileWriter.WriteString(fmt.Sprintf("%v\n", strings.ReplaceAll(line, "'", "\"")))
		}
	}
	// Make sure to flush the log write before we start saving it.
	if err := tmpGeneratedLockfileWriter.Flush(); err != nil {
		return errors.Wrap(err, "failed to flush to temporary lock file")
	}

	// Close the files before we rename them
	if err := tmpGeneratedLockfile.Close(); err != nil {
		return errors.Wrap(err, "failed to close temporary lock file")
	}
	if err := generatedLockfile.Close(); err != nil {
		return errors.Wrap(err, "failed to close existing lock file")
	}

	// Rename the file
	if err := os.Rename(yarnTmpFilePath.ToStringDuringMigration(), lockFilePath.ToStringDuringMigration()); err != nil {
		return errors.Wrap(err, "failed finalize lockfile")
	}
	return nil
}
