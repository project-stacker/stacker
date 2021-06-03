export LDFLAGS=-static
export LDLIBS=$(shell pkg-config --static --libs lxc) -lpthread -ldl

clean:
	-rm lxc-wrapper
