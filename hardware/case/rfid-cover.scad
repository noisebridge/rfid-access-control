$fn=96;
case_fn=6;   // make that 8 for faster development, >= 60 for 'pretty'

epsilon=0.1;

// The RFID 522 board size
rfid_w=40;
rfid_h=60;

oval_ratio=rfid_w/rfid_h;

case_height=12;
// inner volume
v_width=rfid_w + 5;
v_height=rfid_h + 15;
v_depth=case_height-2;

top_radius=0.9*rfid_h;  // the longer part.
base_radius=top_radius + case_height;
logo_size=0.7*top_radius;

cleat_angle=35;

cleat_wall_thick = 1.2;
top_thick=1;
base_thick=1;
clearance=0.5;
logo_imprint=0.3;

module logo() {
    scale([logo_size,logo_size,1]) linear_extrude(height = logo_imprint + 2*epsilon, convexity = 10)
        translate([-0.8,-0.55,0]) import(file = "./Noisebridge-logo.dxf");
}
module board() {
    color("blue") cube([rfid_w,rfid_h,1.5], center=true);
}

module base_plate() {
    scale([oval_ratio,1,1]) cylinder(r=base_radius,h=base_thick);
}

module top_volume() {
    difference() {
	scale([oval_ratio,1,1]) {
	    rotate_extrude(convexity = 10, $fn = case_fn) translate([top_radius,0,0]) circle(r = case_height, $fn = case_fn);
	    translate([0,0,epsilon]) cylinder(r=top_radius, h=case_height-epsilon, $fn=case_fn);
	}
	translate([0,0,-case_height/2]) cube([200,200,case_height+epsilon], center=true);
    }
}

module top_case() {
    difference() {
	minkowski() {
	    top_volume();
	    //sphere(r=top_thick, $fn=12);  // really slow.
	    translate([0,0,top_thick/2+epsilon]) cube([2*top_thick,2*top_thick,top_thick], center=true);  // slow
	}
	top_volume();
	translate([0,0,case_height+top_thick - logo_imprint]) logo();
    }
}

module inner_cleat_volume() {
    b=40;  // cut-away block thickness
    // Mmmh, there certainly must be a simpler way to build a parallelogram
    difference() {
	translate([0, 0, v_depth/2]) cube([v_width, v_height, v_depth], center=true);
	// aligned to the bottom plane
	translate([0,-v_height/2,0]) rotate([-cleat_angle,0,0]) translate([-v_width/2-epsilon,-b,-25]) cube([v_width + 2*epsilon, b, v_depth+50]);

	// aligned to the top plane
	translate([0,v_height/2,v_depth]) rotate([-cleat_angle,0,0]) translate([-v_width/2-epsilon,0,-25]) cube([v_width + 2*epsilon, b, v_depth+50]);
    }
}

module padded_cleat_volume(p=[1,1,1]) {
    minkowski() {
	inner_cleat_volume();
	cube(p, center=true);
    }
}

module inner_frame() {
    difference() {
	padded_cleat_volume(p=[2*cleat_wall_thick,2*cleat_wall_thick,epsilon]);
	translate([0,0,epsilon]) inner_cleat_volume();
	translate([0,0,-epsilon]) inner_cleat_volume();
    }
}

module outer_frame() {
    difference() {
	padded_cleat_volume(p=[4*cleat_wall_thick + 2*clearance,4*cleat_wall_thick + 2*clearance, epsilon]);
	translate([0,0,epsilon]) padded_cleat_volume(p=[2*cleat_wall_thick + 2*clearance,2*cleat_wall_thick+2*clearance,epsilon]);
	translate([0,0,-epsilon]) padded_cleat_volume(p=[2*cleat_wall_thick + 2*clearance,2*cleat_wall_thick+2*clearance,epsilon]);
    }
}

module base_assembly() {
    color("green") {
	translate([0,0,base_thick-epsilon]) inner_frame();
	base_plate();
    }
}

module case_assembly() {
    translate([0,0,case_height - v_depth + epsilon]) color("red") outer_frame();
    top_case();
}

module print() {
    base_assembly();
    translate([2 * oval_ratio * base_radius,0,0]) rotate([0,180,0]) translate([0,0,-case_height - top_thick]) case_assembly();
}

//print();
logo();