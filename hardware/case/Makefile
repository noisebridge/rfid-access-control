all : rfid-case.stl 

rfid-case.stl: Noisebridge-logo.dxf

Noisebridge-logo.dxf:

%.eps: %.svg
	inkscape -E $@ $<

%.dxf: %.eps
	pstoedit -psarg "-r300x300" -dt -f dxf:-polyaslines $< $@

%.stl: %.scad
	openscad -o $@ -d $@.deps $<

clean:
	rm -f $(THING_STL)
