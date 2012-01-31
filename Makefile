include $(GOROOT)/src/Make.inc

all: build

DEPS=\
	src/typeapply\
	src/ldd\
	src/filemarshal\
	src/forth\

DIRS=\
	src/gproc\

clean.deps: $(addsuffix .clean, $(DEPS))
clean.dirs: $(addsuffix .clean, $(DIRS))
build.dirs: $(addsuffix .build, $(DIRS))
build.deps: $(addsuffix .build-dep, $(DEPS))

%.clean:
	+@echo clean $*
	+@$(MAKE) -C $* clean >$*/clean.out 2>&1 || (echo CLEAN FAIL $*; cat $*/clean.out; exit 1)
	+@rm -f $*/clean.out

%.libs:
	+@echo clean-libs
	+@rm -f src/*.a

%.build-dep:
	+@echo build-dep $*
	+@$(MAKE) -C $* >$*/build-dep.out 2>&1 || (echo BUILD-DEP FAIL $*; cat $*/build-dep.out; exit 1)
	+@cp $*/_obj/*.a src/
	+@rm -f $*/build-dep.out

%.build:
	+@echo build $*
	+@$(MAKE) -C $* >$*/build.out 2>&1 || (echo BUILD FAIL $*; cat $*/build.out; exit 1)
	+@cp src/gproc/gproc_$(GOOS)_$(GOARCH) ./gproc
	+@rm -f $*/build.out

clean: clean.deps clean.dirs clean.libs
	+@rm -f ./gproc

echo-dirs:
	@echo $(DIRS)

build: build.deps build.dirs

