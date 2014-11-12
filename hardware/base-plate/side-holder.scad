$fn=8;
epsilon=0.05;
clearance=0.5;
acryl_thick=3 + clearance;
side_high=50;
side_thick=8.5;
side_len=50;
hold_thick=2;
screw_predrill=1.8;

module side(right=1) {
    difference() {
	union() {
	    // Base block.
	    cube([side_high, side_len + hold_thick, side_thick]);
	    // Block that covers where the acryl holder is

	    hull() {
		// Raised part to hold the acrylic
		translate([side_high-3*hold_thick, 0, side_thick+clearance]) cube([3 * hold_thick, side_len + hold_thick, hold_thick]);
		// Lower part, aligned with back wall
		translate([side_high-6*hold_thick, 0, side_thick-hold_thick]) cube([6 * hold_thick, side_len + hold_thick, hold_thick]);
	    }
	}
	
	// Acryl holder.
	translate([side_high - acryl_thick - hold_thick, right ? hold_thick : -epsilon, -epsilon]) cube([acryl_thick, side_len + 2*epsilon, side_thick+clearance+2*epsilon]);
    }
}

// The manual drills are not entirely accurate on the acrylic,
// so adapt to that :)
module left_bracket() {
    difference() {
	translate([0, -(side_len+hold_thick)]) side(right=1);
	// Drill holes.
	translate([-epsilon,-15.96,side_thick-3.869]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);
	translate([-epsilon,-38.61,side_thick-4.755]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);
    }
}

module right_bracket() {
    difference() {
	side(right=0);
	translate([-epsilon,14.83,side_thick-5.562]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);
	translate([-epsilon,39.25,side_thick-3.95]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);
    }
}

left_bracket();
translate([0, 10]) right_bracket();