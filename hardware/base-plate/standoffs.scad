// Standoffs for mounting parts.

epsilon=0.1;
standoff_height=3;
rpi_standoff_height=3.7;
$fn=24;

module drill(d=3.5,h=standoff_height) {
    translate([0,0,-2*h]) cylinder(r=d/2,h=4*h);
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
	cylinder(r=d/2, h=h);
	drill();
    }
}

module full_terminal() {
    // Terminal has two sides
    translate([-5, 0]) terminal();
    translate([5, 0]) terminal();
}

module standoff_collection(h=standoff_height) {
    // Some generic standoffs
    translate([25, 0]) round_standoff(h=h);
    translate([25, 20]) round_standoff(h=h);
    translate([25, -20]) round_standoff(h=h);
}

module sd_holder(h=rpi_standoff_height,m=3) {
    round_standoff(h=h);
    difference() {
	translate([-30.8+3.9-m/2,0.65- m/2,0])  cube([31.3+m,42.5+m,h]);
	translate([-30.8+3.9,    0.65,-epsilon]) cube([31.3,42.5,h+m]);
	drill();
    }
    translate([-30.8+3.9-m/2,4.65-m/2 + 38-15,0]) cube([31.3+m,15+m,0.4]);
}

// http://www.raspberrypi.org/wp-content/uploads/2012/12/Raspberry-Pi-Mounting-Hole-Template.png
module raspi(h=rpi_standoff_height) {
    difference() {
	cube([56,6,h]);
	translate([20, 1, -epsilon]) cube([56-20, 6, h+2*epsilon]);
    }
    translate([56-12.5, 5, 0]) round_standoff(h=h);
    translate([56-16, 0, 0]) cube([10, 3, h]);
    translate([56-3, 0, 0]) cube([3, 6, h]);

    // We need another standoff
    translate([0, 12, 0]) sd_holder();
}

module screw_terminal(w=38,d=10,h=8) {
    cube([w,d,h]);
}

module print_all() {
    relay();
    full_terminal();
    translate([-30, -35, 0]) raspi();
}

//print_all();
screw_terminal();
