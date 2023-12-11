
setup: exampletarball

exampletarball:
	tar zcvf example.tgz *.stacker.yaml


%: %.stacker.yaml exampletarball
	stacker --oci-dir $@ build $(BARG) -f $<
	tree -h $@
	@echo "press enter to run oci-viewer"
	@read a
	~/bin/ociv $@

clean:
	rm -rf 1 2 3 example.tgz
