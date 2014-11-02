// Standoffs for mounting parts.

epsilon=0.1;
standoff_height=3;
$fn=24;

module drill(d=3.5) {
    translate([0,0,-2*standoff_height]) cylinder(r=d/2,h=4*standoff_height);
}

module relay(h=standoff_height) {
    width = 38.6;
    height = 50.6;
    drill_x_top = 33.5;
    drill_x_bot = drill_x_top - 1;
    drill_y = 44.7;
    translate([0,0,h/2]) difference() {
	cube([width, height, h], center=true);
	translate([0,-2,0]) cube([width-11.5, height-6, h+2*epsilon], center=true);
	//translate([0,-5,0]) cube([width-11, height, h+2*epsilon], center=true);
	cube([width-7, height-15, h+2*epsilon], center=true);
	translate([drill_x_top/2, drill_y/2, 0]) drill();
	translate([-drill_x_top/2, drill_y/2, 0]) drill();
	translate([drill_x_bot/2, -drill_y/2, 0]) drill();
	translate([-drill_x_bot/2, -drill_y/2, 0]) drill();
    }
}

module terminal(h=standoff_height) {
    translate([-6/2, -30/2, 0]) difference() {
	cube([6, 30, h]);
	translate([3, 3, 0]) drill();
	translate([3, 21, 0]) drill();
    }
    
}

module round_standoff(h=standoff_height, d=8) {
    difference() {
	cylinder(r=d/2, h=3);
	drill();
    }
}

module full_terminal() {
    // Terminal has two sides
    translate([-5, 0]) terminal();
    translate([5, 0]) terminal();
}

module standoff_collection() {
    // Some generic standoffs
    translate([25, 0]) round_standoff();
    translate([25, 20]) round_standoff();
    translate([25, -20]) round_standoff();
}

relay();
full_terminal();
standoff_collection();   // useful to mount RPi
