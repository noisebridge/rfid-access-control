epsilon=0.1;
acryl_thick=3;
side_high=50;
side_thick=8;
side_len=50;
hold_thick=2;
screw_predrill=1.5;

module side(right=1) {
    difference() {
	cube([side_high, side_len + hold_thick, side_thick]);
	translate([side_high - acryl_thick - hold_thick, right ? hold_thick : -epsilon, hold_thick]) cube([acryl_thick, side_len + 2*epsilon, side_thick]);
	translate([-epsilon,1*side_len/4,side_thick/2]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);
	translate([-epsilon,3*side_len/4,side_thick/2]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);    
    }
}

side(right=1);
translate([0, side_len + 5, 0]) side(right=0);