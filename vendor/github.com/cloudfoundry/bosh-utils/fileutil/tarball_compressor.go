package fileutil

import (
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type tarballCompressor struct {
	cmdRunner boshsys.CmdRunner
	fs        boshsys.FileSystem
}

func NewTarballCompressor(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
) Compressor {
	return tarballCompressor{cmdRunner: cmdRunner, fs: fs}
}

func (c tarballCompressor) CompressFilesInDir(dir string) (string, error) {
	tarball, err := c.fs.TempFile("bosh-platform-disk-TarballCompressor-CompressFilesInDir")
	if err != nil {
		return "", bosherr.WrapError(err, "Creating temporary file for tarball")
	}

	// tar on Windows must use Posix style paths
	tarballPath := strings.Replace(strings.Replace(tarball.Name(), "\\", "/", -1), "C:", "/c", 1)

	_, _, _, err = c.cmdRunner.RunCommand("tar", "czf", tarballPath, "-C", dir, ".")
	if err != nil {
		return "", bosherr.WrapError(err, "Shelling out to tar")
	}

	// Return a Windows path
	return strings.Replace(tarball.Name(), "\\", "/", -1), nil
	//return tarball.Name(), nil
}

func (c tarballCompressor) DecompressFileToDir(tarballPath string, dir string, options CompressorOptions) error {
	sameOwnerOption := "--no-same-owner"
	if options.SameOwner {
		sameOwnerOption = "--same-owner"
	}

	// ensure Windows paths work across shell types
	tarballPath = strings.Replace(tarballPath, "\\", "/", -1)
	dir = strings.Replace(dir, "\\", "/", -1)

	_, _, _, err := c.cmdRunner.RunCommand("tar", sameOwnerOption, "--force-local", "-xzvf", tarballPath, "-C", dir)
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to tar")
	}

	return nil
}

func (c tarballCompressor) CleanUp(tarballPath string) error {
	return c.fs.RemoveAll(tarballPath)
}
