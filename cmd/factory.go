package cmd

import (
	"path/filepath"
	"time"

	bihttpagent "github.com/cloudfoundry/bosh-agent/agentclient/http"
	biblobstore "github.com/cloudfoundry/bosh-init/blobstore"
	bicloud "github.com/cloudfoundry/bosh-init/cloud"
	biconfig "github.com/cloudfoundry/bosh-init/config"
	bicpirel "github.com/cloudfoundry/bosh-init/cpi/release"
	bicrypto "github.com/cloudfoundry/bosh-init/crypto"
	bidepl "github.com/cloudfoundry/bosh-init/deployment"
	bidisk "github.com/cloudfoundry/bosh-init/deployment/disk"
	biinstance "github.com/cloudfoundry/bosh-init/deployment/instance"
	biinstancestate "github.com/cloudfoundry/bosh-init/deployment/instance/state"
	bideplmanifest "github.com/cloudfoundry/bosh-init/deployment/manifest"
	bideplrel "github.com/cloudfoundry/bosh-init/deployment/release"
	bisshtunnel "github.com/cloudfoundry/bosh-init/deployment/sshtunnel"
	bivm "github.com/cloudfoundry/bosh-init/deployment/vm"
	biindex "github.com/cloudfoundry/bosh-init/index"
	biinstall "github.com/cloudfoundry/bosh-init/installation"
	biinstallmanifest "github.com/cloudfoundry/bosh-init/installation/manifest"
	bitarball "github.com/cloudfoundry/bosh-init/installation/tarball"
	biregistry "github.com/cloudfoundry/bosh-init/registry"
	birel "github.com/cloudfoundry/bosh-init/release"
	birelsetmanifest "github.com/cloudfoundry/bosh-init/release/set/manifest"
	bistatepkg "github.com/cloudfoundry/bosh-init/state/pkg"
	bistemcell "github.com/cloudfoundry/bosh-init/stemcell"
	bitemplate "github.com/cloudfoundry/bosh-init/templatescompiler"
	bitemplateerb "github.com/cloudfoundry/bosh-init/templatescompiler/erbrenderer"
	biui "github.com/cloudfoundry/bosh-init/ui"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	bihttpclient "github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	"github.com/pivotal-golang/clock"
)

type Factory interface {
	CreateCommand(name string) (Cmd, error)
}

type factory struct {
	commands               CommandList
	fs                     boshsys.FileSystem
	ui                     biui.UI
	timeService            clock.Clock
	logger                 boshlog.Logger
	uuidGenerator          boshuuid.Generator
	workspaceRootPath      string
	runner                 boshsys.CmdRunner
	compressor             boshcmd.Compressor
	agentClientFactory     bihttpagent.AgentClientFactory
	registryServerManager  biregistry.ServerManager
	sshTunnelFactory       bisshtunnel.Factory
	instanceFactory        biinstance.Factory
	instanceManagerFactory biinstance.ManagerFactory
	deploymentFactory      bidepl.Factory
	blobstoreFactory       biblobstore.Factory
	eventLogger            biui.Stage
	releaseExtractor       birel.Extractor
	releaseManager         birel.Manager
	releaseSetParser       birelsetmanifest.Parser
	releaseJobResolver     bideplrel.JobResolver
	installationParser     biinstallmanifest.Parser
	deploymentParser       bideplmanifest.Parser
	releaseSetValidator    birelsetmanifest.Validator
	installationValidator  biinstallmanifest.Validator
	deploymentValidator    bideplmanifest.Validator
	cloudFactory           bicloud.Factory
	stateBuilderFactory    biinstancestate.BuilderFactory
	compiledPackageRepo    bistatepkg.CompiledPackageRepo
	tarballProvider        bitarball.Provider
	cpiReleaseValidator    *bicpirel.Validator
}

func NewFactory(
	fs boshsys.FileSystem,
	ui biui.UI,
	timeService clock.Clock,
	logger boshlog.Logger,
	uuidGenerator boshuuid.Generator,
	workspaceRootPath string,
) Factory {
	f := &factory{
		fs:                fs,
		ui:                ui,
		timeService:       timeService,
		logger:            logger,
		uuidGenerator:     uuidGenerator,
		workspaceRootPath: workspaceRootPath,
	}
	f.commands = CommandList{
		"deploy":  f.createDeployCmd,
		"delete":  f.createDeleteCmd,
		"help":    f.createHelpCmd,
		"version": f.createVersionCmd,
	}
	return f
}

type CommandList map[string](func() (Cmd, error))

func (cl CommandList) Create(name string) (Cmd, error) {
	if cl[name] == nil {
		return nil, bosherr.Errorf("Command '%s' unknown. See 'bosh-init help'", name)
	}

	return cl[name]()
}

func (f *factory) CreateCommand(name string) (Cmd, error) {
	return f.commands.Create(name)
}

func (f *factory) createDeployCmd() (Cmd, error) {
	getter := func(deploymentManifestPath string) (DeploymentPreparer, error) {
		f := &deploymentManagerFactory2{f: f, deploymentManifestPath: deploymentManifestPath}
		deploymentPreparer, err := f.loadDeploymentPreparer()
		if err != nil {
			return deploymentPreparer, err
		}

		return deploymentPreparer, nil
	}
	return NewDeployCmd(f.ui, f.fs, f.logger, getter), nil
}

func (f *factory) createDeleteCmd() (Cmd, error) {
	getter := func(deploymentManifestPath string) (DeploymentDeleter, error) {
		f := &deploymentManagerFactory2{f: f, deploymentManifestPath: deploymentManifestPath}
		deploymentDeleter, err := f.loadDeploymentDeleter()
		if err != nil {
			return deploymentDeleter, err
		}
		return deploymentDeleter, nil
	}

	return NewDeleteCmd(f.ui, f.fs, f.logger, getter), nil
}

func (f *factory) createHelpCmd() (Cmd, error) {
	return NewHelpCmd(f.ui, f.commands), nil
}

func (f *factory) createVersionCmd() (Cmd, error) {
	return NewVersionCmd(f.ui), nil
}

func (f *factory) loadCMDRunner() boshsys.CmdRunner {
	if f.runner != nil {
		return f.runner
	}
	f.runner = boshsys.NewExecCmdRunner(f.logger)
	return f.runner
}

func (f *factory) loadCompressor() boshcmd.Compressor {
	if f.compressor != nil {
		return f.compressor
	}
	f.compressor = boshcmd.NewTarballCompressor(f.loadCMDRunner(), f.fs)
	return f.compressor
}

func (f *factory) loadCompiledPackageRepo() bistatepkg.CompiledPackageRepo {
	if f.compiledPackageRepo != nil {
		return f.compiledPackageRepo
	}

	index := biindex.NewInMemoryIndex()
	f.compiledPackageRepo = bistatepkg.NewCompiledPackageRepo(index)
	return f.compiledPackageRepo
}

func (f *factory) loadRegistryServerManager() biregistry.ServerManager {
	if f.registryServerManager != nil {
		return f.registryServerManager
	}

	f.registryServerManager = biregistry.NewServerManager(f.logger)
	return f.registryServerManager
}

func (f *factory) loadSSHTunnelFactory() bisshtunnel.Factory {
	if f.sshTunnelFactory != nil {
		return f.sshTunnelFactory
	}

	f.sshTunnelFactory = bisshtunnel.NewFactory(f.logger)
	return f.sshTunnelFactory
}

func (f *factory) loadReleaseJobResolver() bideplrel.JobResolver {
	if f.releaseJobResolver != nil {
		return f.releaseJobResolver
	}

	f.releaseJobResolver = bideplrel.NewJobResolver(f.loadReleaseManager())
	return f.releaseJobResolver
}

func (f *factory) loadDeploymentFactory() bidepl.Factory {
	if f.deploymentFactory != nil {
		return f.deploymentFactory
	}

	pingTimeout := 10 * time.Second
	pingDelay := 500 * time.Millisecond

	f.deploymentFactory = bidepl.NewFactory(
		pingTimeout,
		pingDelay,
	)
	return f.deploymentFactory
}

func (f *factory) loadAgentClientFactory() bihttpagent.AgentClientFactory {
	if f.agentClientFactory != nil {
		return f.agentClientFactory
	}

	f.agentClientFactory = bihttpagent.NewAgentClientFactory(1*time.Second, f.logger)
	return f.agentClientFactory
}

func (f *factory) loadBlobstoreFactory() biblobstore.Factory {
	if f.blobstoreFactory != nil {
		return f.blobstoreFactory
	}

	f.blobstoreFactory = biblobstore.NewBlobstoreFactory(f.uuidGenerator, f.fs, f.logger)
	return f.blobstoreFactory
}
func (f *factory) loadCPIReleaseValidator() bicpirel.Validator {
	if f.cpiReleaseValidator != nil {
		return *f.cpiReleaseValidator
	}
	x := bicpirel.NewValidator()
	f.cpiReleaseValidator = &x
	return *f.cpiReleaseValidator
}

func (f *factory) loadReleaseExtractor() birel.Extractor {
	if f.releaseExtractor != nil {
		return f.releaseExtractor
	}

	releaseValidator := birel.NewValidator(f.fs)
	f.releaseExtractor = birel.NewExtractor(f.fs, f.loadCompressor(), releaseValidator, f.logger)
	return f.releaseExtractor
}

func (f *factory) loadTarballProvider() bitarball.Provider {
	if f.tarballProvider != nil {
		return f.tarballProvider
	}

	tarballCacheBasePath := filepath.Join(f.workspaceRootPath, "downloads")
	tarballCache := bitarball.NewCache(tarballCacheBasePath, f.fs, f.logger)
	httpClient := bihttpclient.NewHTTPClient(bitarball.HTTPClient, f.logger)
	sha1Calculator := bicrypto.NewSha1Calculator(f.fs)
	f.tarballProvider = bitarball.NewProvider(tarballCache, f.fs, httpClient, sha1Calculator, 3, 500*time.Millisecond, f.logger)
	return f.tarballProvider
}

func (f *factory) loadReleaseManager() birel.Manager {
	if f.releaseManager != nil {
		return f.releaseManager
	}

	f.releaseManager = birel.NewManager(f.logger)
	return f.releaseManager
}

func (f *factory) loadReleaseSetParser() birelsetmanifest.Parser {
	if f.releaseSetParser != nil {
		return f.releaseSetParser
	}

	f.releaseSetParser = birelsetmanifest.NewParser(f.fs, f.logger, f.loadReleaseSetValidator())
	return f.releaseSetParser
}

func (f *factory) loadInstallationParser() biinstallmanifest.Parser {
	if f.installationParser != nil {
		return f.installationParser
	}

	uuidGenerator := boshuuid.NewGenerator()
	f.installationParser = biinstallmanifest.NewParser(f.fs, uuidGenerator, f.logger, f.loadInstallationValidator())
	return f.installationParser
}

func (f *factory) loadDeploymentParser() bideplmanifest.Parser {
	if f.deploymentParser != nil {
		return f.deploymentParser
	}

	f.deploymentParser = bideplmanifest.NewParser(f.fs, f.logger)
	return f.deploymentParser
}

func (f *factory) loadInstallationValidator() biinstallmanifest.Validator {
	if f.installationValidator != nil {
		return f.installationValidator
	}

	f.installationValidator = biinstallmanifest.NewValidator(f.logger)
	return f.installationValidator
}

func (f *factory) loadDeploymentValidator() bideplmanifest.Validator {
	if f.deploymentValidator != nil {
		return f.deploymentValidator
	}

	f.deploymentValidator = bideplmanifest.NewValidator(f.logger)
	return f.deploymentValidator
}

func (f *factory) loadReleaseSetValidator() birelsetmanifest.Validator {
	if f.releaseSetValidator != nil {
		return f.releaseSetValidator
	}

	f.releaseSetValidator = birelsetmanifest.NewValidator(f.logger)
	return f.releaseSetValidator
}

type deploymentManagerFactory2 struct {
	f                             *factory
	deploymentManifestPath        string
	deploymentStateService        biconfig.DeploymentStateService
	legacyDeploymentStateMigrator biconfig.LegacyDeploymentStateMigrator
	vmRepo                        biconfig.VMRepo
	stemcellRepo                  biconfig.StemcellRepo
	diskRepo                      biconfig.DiskRepo
	diskDeployer                  bivm.DiskDeployer
	diskManagerFactory            bidisk.ManagerFactory
	deploymentManagerFactory      bidepl.ManagerFactory
	vmManagerFactory              bivm.ManagerFactory
	stemcellManagerFactory        bistemcell.ManagerFactory
	installerFactory              biinstall.InstallerFactory
	deployer                      bidepl.Deployer
	rubyRelease                   bitemplateerb.RubyRelease
}

func (d *deploymentManagerFactory2) loadDeploymentPreparer() (DeploymentPreparer, error) {
	deploymentRepo := biconfig.NewDeploymentRepo(d.loadDeploymentStateService())
	releaseRepo := biconfig.NewReleaseRepo(d.loadDeploymentStateService(), d.f.uuidGenerator)
	sha1Calculator := bicrypto.NewSha1Calculator(d.f.fs)
	deploymentRecord := bidepl.NewRecord(deploymentRepo, releaseRepo, d.loadStemcellRepo(), sha1Calculator)
	cpiInstaller, err := d.loadCpiInstaller()
	if err != nil {
		return DeploymentPreparer{}, err
	}

	return NewDeploymentPreparer(
		d.f.ui,
		d.f.logger,
		"DeploymentPreparer",
		d.loadDeploymentStateService(),
		d.loadLegacyDeploymentStateMigrator(),
		d.f.loadReleaseManager(),
		deploymentRecord,
		d.loadCloudFactory(),
		d.loadStemcellManagerFactory(),
		d.f.loadAgentClientFactory(),
		d.loadVMManagerFactory(),
		d.f.loadBlobstoreFactory(),
		d.loadDeployer(),
		d.deploymentManifestPath,
		cpiInstaller,
		d.loadReleaseFetcher(),
		d.loadStemcellFetcher(),
		d.loadReleaseSetAndInstallationManifestParser(),
		d.loadDeploymentManifestParser(),
		NewTempRootConfigurator(d.f.fs),
		d.loadTargetProvider(),
	), nil
}

func (d *deploymentManagerFactory2) loadDeploymentDeleter() (DeploymentDeleter, error) {
	cpiInstaller, err := d.loadCpiInstaller()
	if err != nil {
		return nil, err
	}
	return NewDeploymentDeleter(
		d.f.ui,
		"DeploymentDeleter",
		d.f.logger,
		d.loadDeploymentStateService(),
		d.f.loadReleaseManager(),
		d.loadCloudFactory(),
		d.f.loadAgentClientFactory(),
		d.f.loadBlobstoreFactory(),
		d.loadDeploymentManagerFactory(),
		d.deploymentManifestPath,
		cpiInstaller,
		d.loadCpiUninstaller(),
		d.loadReleaseFetcher(),
		d.loadReleaseSetAndInstallationManifestParser(),
		NewTempRootConfigurator(d.f.fs),
		d.loadTargetProvider(),
	), nil
}

func (d *deploymentManagerFactory2) loadInstanceManagerFactory() biinstance.ManagerFactory {
	if d.f.instanceManagerFactory != nil {
		return d.f.instanceManagerFactory
	}

	d.f.instanceManagerFactory = biinstance.NewManagerFactory(
		d.f.loadSSHTunnelFactory(),
		d.loadInstanceFactory(),
		d.f.logger,
	)
	return d.f.instanceManagerFactory
}

func (d *deploymentManagerFactory2) loadInstanceFactory() biinstance.Factory {
	if d.f.instanceFactory != nil {
		return d.f.instanceFactory
	}

	d.f.instanceFactory = biinstance.NewFactory(
		d.loadBuilderFactory(),
	)
	return d.f.instanceFactory
}

func (d *deploymentManagerFactory2) loadRubyRelease() bitemplateerb.RubyRelease {
	if d.rubyRelease != nil {
		return d.rubyRelease
	}
	target, _ := d.loadTargetProvider().NewTarget()
	d.rubyRelease = bitemplateerb.NewRubyRelease(target.PackagesPath(), d.f.fs, d.f.logger)
	return d.rubyRelease
}

func (d *deploymentManagerFactory2) loadBuilderFactory() biinstancestate.BuilderFactory {
	if d.f.stateBuilderFactory != nil {
		return d.f.stateBuilderFactory
	}

	rubyRelease := d.loadRubyRelease()
	erbRenderer := bitemplateerb.NewERBRenderer(d.f.fs, d.f.loadCMDRunner(), rubyRelease, d.f.logger)
	jobRenderer := bitemplate.NewJobRenderer(erbRenderer, d.f.fs, d.f.logger)
	jobListRenderer := bitemplate.NewJobListRenderer(jobRenderer, d.f.logger)

	sha1Calculator := bicrypto.NewSha1Calculator(d.f.fs)

	renderedJobListCompressor := bitemplate.NewRenderedJobListCompressor(
		d.f.fs,
		d.f.loadCompressor(),
		sha1Calculator,
		d.f.logger,
	)

	d.f.stateBuilderFactory = biinstancestate.NewBuilderFactory(
		d.f.loadCompiledPackageRepo(),
		d.f.loadReleaseJobResolver(),
		jobListRenderer,
		renderedJobListCompressor,
		d.f.logger,
	)
	return d.f.stateBuilderFactory
}

func (d *deploymentManagerFactory2) loadCloudFactory() bicloud.Factory {
	if d.f.cloudFactory != nil {
		return d.f.cloudFactory
	}

	d.f.cloudFactory = bicloud.NewFactory(d.f.fs, d.f.loadCMDRunner(), d.f.logger)
	return d.f.cloudFactory
}

func (d *deploymentManagerFactory2) loadDeploymentStateService() biconfig.DeploymentStateService {
	if d.deploymentStateService != nil {
		return d.deploymentStateService
	}

	d.deploymentStateService = biconfig.NewFileSystemDeploymentStateService(
		d.f.fs,
		d.f.uuidGenerator,
		d.f.logger,
		biconfig.DeploymentStatePath(d.deploymentManifestPath),
	)
	return d.deploymentStateService
}

func (d *deploymentManagerFactory2) loadLegacyDeploymentStateMigrator() biconfig.LegacyDeploymentStateMigrator {
	if d.legacyDeploymentStateMigrator != nil {
		return d.legacyDeploymentStateMigrator
	}

	d.legacyDeploymentStateMigrator = biconfig.NewLegacyDeploymentStateMigrator(
		d.loadDeploymentStateService(),
		d.f.fs,
		d.f.uuidGenerator,
		d.f.logger,
	)
	return d.legacyDeploymentStateMigrator
}

func (d *deploymentManagerFactory2) loadStemcellRepo() biconfig.StemcellRepo {
	if d.stemcellRepo != nil {
		return d.stemcellRepo
	}
	d.stemcellRepo = biconfig.NewStemcellRepo(d.loadDeploymentStateService(), d.f.uuidGenerator)
	return d.stemcellRepo
}

func (d *deploymentManagerFactory2) loadVMRepo() biconfig.VMRepo {
	if d.vmRepo != nil {
		return d.vmRepo
	}
	d.vmRepo = biconfig.NewVMRepo(d.loadDeploymentStateService())
	return d.vmRepo
}

func (d *deploymentManagerFactory2) loadDiskRepo() biconfig.DiskRepo {
	if d.diskRepo != nil {
		return d.diskRepo
	}
	d.diskRepo = biconfig.NewDiskRepo(d.loadDeploymentStateService(), d.f.uuidGenerator)
	return d.diskRepo
}

func (d *deploymentManagerFactory2) loadDiskDeployer() bivm.DiskDeployer {
	if d.diskDeployer != nil {
		return d.diskDeployer
	}

	d.diskDeployer = bivm.NewDiskDeployer(d.loadDiskManagerFactory(), d.loadDiskRepo(), d.f.logger)
	return d.diskDeployer
}

func (d *deploymentManagerFactory2) loadDiskManagerFactory() bidisk.ManagerFactory {
	if d.diskManagerFactory != nil {
		return d.diskManagerFactory
	}

	d.diskManagerFactory = bidisk.NewManagerFactory(d.loadDiskRepo(), d.f.logger)
	return d.diskManagerFactory
}

func (d *deploymentManagerFactory2) loadDeploymentManagerFactory() bidepl.ManagerFactory {
	if d.deploymentManagerFactory != nil {
		return d.deploymentManagerFactory
	}

	d.deploymentManagerFactory = bidepl.NewManagerFactory(
		d.loadVMManagerFactory(),
		d.loadInstanceManagerFactory(),
		d.loadDiskManagerFactory(),
		d.loadStemcellManagerFactory(),
		d.f.loadDeploymentFactory(),
	)
	return d.deploymentManagerFactory
}

func (d *deploymentManagerFactory2) loadVMManagerFactory() bivm.ManagerFactory {
	if d.vmManagerFactory != nil {
		return d.vmManagerFactory
	}

	d.vmManagerFactory = bivm.NewManagerFactory(
		d.loadVMRepo(),
		d.loadStemcellRepo(),
		d.loadDiskDeployer(),
		d.f.uuidGenerator,
		d.f.fs,
		d.f.logger,
	)
	return d.vmManagerFactory
}

func (d *deploymentManagerFactory2) loadStemcellManagerFactory() bistemcell.ManagerFactory {
	if d.stemcellManagerFactory != nil {
		return d.stemcellManagerFactory
	}

	d.stemcellManagerFactory = bistemcell.NewManagerFactory(d.loadStemcellRepo())
	return d.stemcellManagerFactory
}

func (d *deploymentManagerFactory2) loadDeployer() bidepl.Deployer {
	if d.deployer != nil {
		return d.deployer
	}

	d.deployer = bidepl.NewDeployer(
		d.loadVMManagerFactory(),
		d.loadInstanceManagerFactory(),
		d.f.loadDeploymentFactory(),
		d.f.logger,
	)
	return d.deployer
}

func (d *deploymentManagerFactory2) loadInstallerFactory() biinstall.InstallerFactory {
	if d.installerFactory != nil {
		return d.installerFactory
	}

	d.installerFactory = biinstall.NewInstallerFactory(
		d.f.ui,
		d.f.loadCMDRunner(),
		d.f.loadCompressor(),
		d.f.loadReleaseJobResolver(),
		d.f.uuidGenerator,
		d.f.loadRegistryServerManager(),
		d.f.logger,
		d.f.fs,
	)
	return d.installerFactory
}

func (d *deploymentManagerFactory2) loadTargetProvider() biinstall.TargetProvider {
	return biinstall.NewTargetProvider(
		d.loadDeploymentStateService(),
		d.f.uuidGenerator,
		filepath.Join(d.f.workspaceRootPath, "installations"),
	)
}

func (d *deploymentManagerFactory2) loadCpiUninstaller() biinstall.Uninstaller {
	return biinstall.NewUninstaller(d.f.fs, d.f.logger)
}

func (d *deploymentManagerFactory2) loadCpiInstaller() (bicpirel.CpiInstaller, error) {
	return bicpirel.CpiInstaller{
		ReleaseManager:   d.f.loadReleaseManager(),
		InstallerFactory: d.loadInstallerFactory(),
		Validator:        bicpirel.NewValidator(),
	}, nil
}

func (d *deploymentManagerFactory2) loadReleaseFetcher() birel.Fetcher {
	return birel.NewFetcher(
		d.f.loadTarballProvider(),
		d.f.loadReleaseExtractor(),
		d.f.loadReleaseManager(),
	)
}

func (d *deploymentManagerFactory2) loadStemcellFetcher() bistemcell.Fetcher {
	stemcellReader := bistemcell.NewReader(d.f.loadCompressor(), d.f.fs)
	stemcellExtractor := bistemcell.NewExtractor(stemcellReader, d.f.fs)

	return bistemcell.Fetcher{
		TarballProvider:   d.f.loadTarballProvider(),
		StemcellExtractor: stemcellExtractor,
	}
}

func (d *deploymentManagerFactory2) loadReleaseSetAndInstallationManifestParser() ReleaseSetAndInstallationManifestParser {
	return ReleaseSetAndInstallationManifestParser{
		ReleaseSetParser:   d.f.loadReleaseSetParser(),
		InstallationParser: d.f.loadInstallationParser(),
	}
}

func (d *deploymentManagerFactory2) loadDeploymentManifestParser() DeploymentManifestParser {
	return DeploymentManifestParser{
		DeploymentParser:    d.f.loadDeploymentParser(),
		DeploymentValidator: d.f.loadDeploymentValidator(),
		ReleaseManager:      d.f.loadReleaseManager(),
	}
}
