OUTPUT ?= lxc-wrapper

$(OUTPUT): lxc-wrapper.c
	$(CC) $(CPPFLAGS) $(CFLAGS) $(LDFLAGS) -o $@ $< $(LDLIBS)

.PHONY: clean
clean:
	-rm -f lxc-wrapper lxc-wrapper-host lxc-wrapper-static
