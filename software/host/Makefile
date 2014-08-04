
LIBS=lcd/libsimplelcd.a
OBJECTS=main.o

CXXFLAGS=-Wall
LDFLAGS=-Llcd -lsimplelcd

all : access-control

access-control: $(OBJECTS) $(LIBS)
	g++ -o $@ $(OBJECTS) $(LDFLAGS)

$(LIBS):
	make -C lcd

clean:
	rm -f $(OBJECTS) access-control
