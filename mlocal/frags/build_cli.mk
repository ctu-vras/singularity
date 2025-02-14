# This file contains all of the rules for building the singularity CLI binary

# singularity build config
singularity_build_config := $(SOURCEDIR)/internal/pkg/buildcfg/config.go
$(singularity_build_config): $(BUILDDIR_ABSPATH)/config.h $(SOURCEDIR)/scripts/go-generate
	$(V)rm -f $(singularity_build_config)
	$(V) cd $(SOURCEDIR)/internal/pkg/buildcfg && $(SOURCEDIR)/scripts/go-generate

CLEANFILES += $(singularity_build_config)

# contain singularity_SOURCE variable list
singularity_deps := $(BUILDDIR_ABSPATH)/singularity.d

-include $(singularity_deps)

$(singularity_deps): $(GO_MODFILES)
	@echo " GEN GO DEP" $@
	$(V)cd $(SOURCEDIR) && ./makeit/gengodep -v3 "$(GO)" "singularity_SOURCE" "$(GO_TAGS)" "$@" "$(SOURCEDIR)/cmd/singularity"

# Look at dependencies file changes via singularity_deps
# because it means that a module was updated.
singularity := $(BUILDDIR_ABSPATH)/singularity
$(singularity): $(singularity_build_config) $(singularity_deps) $(singularity_SOURCE)
	@echo " GO" $@; echo "    [+] GO_TAGS" \"$(GO_TAGS)\"
	$(V)cd $(SOURCEDIR) && $(GO) build $(GO_MODFLAGS) $(GO_BUILDMODE) -tags "$(GO_TAGS)" $(GO_LDFLAGS) \
		-o $@ ./cmd/singularity

singularity_INSTALL := $(DESTDIR)$(BINDIR)/singularity
$(singularity_INSTALL): $(singularity)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0755 $(singularity) $(singularity_INSTALL) # set cp to install

CLEANFILES += $(singularity)
INSTALLFILES += $(singularity_INSTALL)
ALL += $(singularity)


# bash_completion file
bash_completion :=  $(BUILDDIR_ABSPATH)/bash-completion/completions/singularity
$(bash_completion): $(singularity_build_config)
	@echo " GEN" $@
	$(V)rm -f $@
	$(V)mkdir -p $(@D)
	$(V)cd $(SOURCEDIR) && $(GO) run $(GO_MODFLAGS) -tags "$(GO_TAGS)" \
		cmd/bash_completion/bash_completion.go $@

bash_completion_INSTALL := $(DESTDIR)$(DATADIR)/bash-completion/completions/singularity
$(bash_completion_INSTALL): $(bash_completion)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

CLEANFILES += $(bash_completion)
INSTALLFILES += $(bash_completion_INSTALL)
ALL += $(bash_completion)


# singularity.conf file
config := $(BUILDDIR_ABSPATH)/singularity.conf
config_INSTALL := $(DESTDIR)$(SYSCONFDIR)/singularity/singularity.conf
# override this to empty to avoid merging old configuration settings
old_config := $(config_INSTALL)

$(config): $(singularity_build_config) $(SOURCEDIR)/etc/conf/gen.go $(SOURCEDIR)/pkg/runtime/engine/singularity/config/config.go
	@echo " GEN $@`if [ -n "$(old_config)" ]; then echo " from $(old_config)"; fi`"
	$(V)cd $(SOURCEDIR) && $(GO) run $(GO_MODFLAGS) etc/conf/gen.go \
		$(old_config) $(config)

$(config_INSTALL): $(config)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(config_INSTALL)
ALL += $(config)

# remote config file
remote_config := $(SOURCEDIR)/etc/remote.yaml

remote_config_INSTALL := $(DESTDIR)$(SYSCONFDIR)/singularity/remote.yaml
$(remote_config_INSTALL): $(remote_config)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(remote_config_INSTALL)

man_pages := $(BUILDDIR_ABSPATH)$(MANDIR)/man1
$(man_pages): singularity
	@echo " MAN" $@
	mkdir -p $@
	$(V)cd $(SOURCEDIR) && $(GO) run $(GO_MODFLAGS) -tags "$(GO_TAGS)" \
		cmd/docs/docs.go man --dir $@

man_pages_INSTALL := $(DESTDIR)$(MANDIR)/man1
$(man_pages_INSTALL): $(man_pages)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $@
	$(V)install -m 0644 -t $@ $(man_pages)/*

INSTALLFILES += $(man_pages_INSTALL)
ALL += $(man_pages)
