all: standoffs.stl side-holder.stl

standoffs.stl:
side-holder.stl:

%.stl: %.scad
	openscad -o $@ -d $@.deps $<
