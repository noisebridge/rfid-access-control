$fn=24;
epsilon=0.05;
clearance=0.3;

drill_x = 113;
drill_y = 31;

border=1.5;
drill_dia=3;
stud_dia=2*drill_dia;
nut_thick=2;        // The nut we slide in sideways
nut_dia=5;
bolt_top_flap=0.4;  // flap thickness covering up screw.

lcd_w=103 + clearance;
lcd_h=29.2 + clearance;
lcd_t=9.4;  // thickness
lcd_dx=2.3;
lcd_dy=(drill_y - lcd_h)/2;

module lcd() {
    translate([lcd_dx, lcd_dy, -epsilon]) {
	cube([lcd_w, lcd_h, lcd_t]);
	translate([0, 0, lcd_t-epsilon]) cube([lcd_w, lcd_h, 2]);
    }
}

module block() {
    translate([-stud_dia/2,-stud_dia/2,0]) {
	difference() {
	    cube([drill_x+stud_dia, drill_y+stud_dia, lcd_t]);
	    translate([border,border,-epsilon]) cube([drill_x+stud_dia-2*border, drill_y+stud_dia-2*border, lcd_t-border]);
	}
    }
    board_studs();
}

module bolt() {
    translate([0,0,lcd_t/2]) cylinder(r=nut_dia/2, h=nut_thick);
}

// Stud around drill.
module drill_stud() {
    difference() {
	cylinder(r=stud_dia/2, h=lcd_t);
	bolt();
    }
}

// The actual drill. Applied at the end.
module drill(angle=0) {
    translate([0,0,-epsilon]) cylinder(r=drill_dia/2, h=lcd_t - bolt_top_flap);
    // A slot to slide in the 
    rotate([0,0,angle]) translate([0,-nut_dia/2,lcd_t/2]) cube([2*stud_dia,nut_dia,nut_thick]);
}

module board_studs() {
    translate([0, 0]) drill_stud();
    translate([drill_x, 0]) drill_stud();
    translate([drill_x, drill_y]) drill_stud();
    translate([0, drill_y]) drill_stud();
}

module board_drill() {
    translate([0, 0]) drill(225);
    translate([drill_x, 0]) drill(315);
    translate([drill_x, drill_y]) drill(45);
    translate([0, drill_y]) drill(135);
}

module display_holder() {
    difference() {
	block();
	board_drill();
	lcd();
    }
}

rotate([180,0,0]) display_holder();
//display_holder();
