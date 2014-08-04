
LIBS=lcd/libsimplelcd.a
OBJECTS=main.o

CXXFLAGS=-Wall
LDFLAGS=-Llcd -lsimplelcd

access-control: $(OBJECTS) $(LIBS)
	g++ -o $@ $(OBJECTS) $(LDFLAGS)
